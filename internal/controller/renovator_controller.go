package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
)

// RenovatorReconciler reconciles a Renovator object
type RenovatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

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
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RenovatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.Renovator{}).
		Named("renovator").
		Complete(r)
}
