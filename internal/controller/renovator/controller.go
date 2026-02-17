package renovator

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/controller"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const ControllerName = "renovator"

// Reconciler reconciles a Renovator object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//nolint:lll
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovators/finalizers,verbs=update
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=discoveries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=discoveries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=discoveries/finalizers,verbs=update
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=runners,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=runners/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=runners/finalizers,verbs=update
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovateconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovateconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovateconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.V(1).Info("Reconciling object", "object", req.NamespacedName)

	rr := &renovatev1beta1.Renovator{}

	if err := r.Get(ctx, req.NamespacedName, rr); err != nil {
		if api_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	renovator, err := renovator.NewReconciler(ctx, r.Client, r.Scheme, rr)
	if err != nil {
		return ctrl.Result{}, err
	}

	res, err := renovator.Reconcile(ctx)
	if err != nil {
		return controller.HandleReconcileResult(res, err)
	}

	return *res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.Renovator{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldAnn := e.ObjectOld.GetAnnotations()
					newAnn := e.ObjectNew.GetAnnotations()

					return renovator.HasRenovatorOperation(newAnn) &&
						!renovator.HasRenovatorOperation(oldAnn)
				},
				CreateFunc:  func(_ event.CreateEvent) bool { return true },
				DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
				GenericFunc: func(_ event.GenericEvent) bool { return false },
			},
		)).
		Owns(&renovatev1beta1.RenovateConfig{}).
		Owns(&renovatev1beta1.Discovery{}).
		Owns(&renovatev1beta1.Runner{}).
		Named(ControllerName).
		Complete(r)
}
