package controller

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/reconciler/discovery"
	"github.com/thegeeklab/renovate-operator/pkg/reconciler/runner"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// RenovatorReconciler reconciles a Renovator object.
type RenovatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//nolint:lll
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovators/finalizers,verbs=update
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *RenovatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)
	ctxLogger.V(1).Info(fmt.Sprintf("reconciling object %#q", req.NamespacedName))

	renovatorRes := &renovatev1beta1.Renovator{}

	err := r.Get(ctx, req.NamespacedName, renovatorRes)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if res, err := discovery.Reconcile(ctx, r.Client, r.Scheme, req, renovatorRes); err != nil {
		return handleReconcileResult(res, err)
	}

	if res, err := runner.Reconcile(ctx, r.Client, r.Scheme, req, renovatorRes); err != nil {
		return handleReconcileResult(res, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RenovatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.Renovator{}).
		Watches(&renovatev1beta1.GitRepo{}, handler.EnqueueRequestForOwner(
			r.Scheme,
			mgr.GetRESTMapper(),
			&renovatev1beta1.Renovator{},
		)).
		Named("renovator").
		Complete(r)
}

func handleReconcileResult(res *ctrl.Result, err error) (ctrl.Result, error) {
	if err != nil {
		if res != nil {
			return *res, err
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
