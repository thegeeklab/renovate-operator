package controller

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/worker"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// RenovatorReconciler reconciles a Renovator object.
type RenovatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovators/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Renovator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *RenovatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := logf.FromContext(ctx)
	reqLogger.V(1).Info(fmt.Sprintf("reconciling object %#q", req.NamespacedName))

	renovatorInst := &renovatev1beta1.Renovator{}

	err := r.Get(ctx, req.NamespacedName, renovatorInst)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	namespace := req.NamespacedName.Namespace

	if renovatorInst.Spec.DiscoveryRef.Namespace != "" {
		namespace = renovatorInst.Spec.DiscoveryRef.Namespace
	}

	// Get the referenced Discovery
	discoveryInst := &renovatev1beta1.Discovery{}
	discoveryInstKey := types.NamespacedName{
		Name:      renovatorInst.Spec.DiscoveryRef.Name,
		Namespace: namespace,
	}

	if err := r.Get(ctx, discoveryInstKey, discoveryInst); err != nil {
		return ctrl.Result{}, err
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(discoveryInst, renovatorInst, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Update the Renovator with the owner reference
	if err := r.Update(ctx, renovatorInst); err != nil {
		return ctrl.Result{}, err
	}

	worker := worker.New(r.Client, req, renovatorInst, discoveryInst, r.Scheme)

	result, err := worker.Reconcile(ctx)
	if err != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RenovatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.Renovator{}).
		Named("renovator").
		Complete(r)
}
