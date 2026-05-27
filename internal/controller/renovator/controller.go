package renovator

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/controller"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
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
	Scheme        *runtime.Scheme
	EventRecorder events.EventRecorder
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

	original := rr.DeepCopy()

	outcome := r.reconcile(ctx, rr)
	controller.FinalizeStatus(ctx, r.Client, r.EventRecorder, original, rr, outcome,
		controller.FinalizeStatusOptions{SuccessMessage: "Renovator reconciled successfully"})

	return controller.HandleReconcileResult(outcome.Result, outcome.Err)
}

// reconcile runs the Renovator reconciliation pipeline.
func (r *Reconciler) reconcile(
	ctx context.Context, rr *renovatev1beta1.Renovator,
) controller.Outcome {
	componentReconciler, err := renovator.NewReconciler(ctx, r.Client, r.Scheme, rr)
	if err != nil {
		return controller.Outcome{Err: err}
	}

	res, err := componentReconciler.Reconcile(ctx)

	return controller.Outcome{Result: res, Err: err}
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.EventRecorder = mgr.GetEventRecorder(ControllerName)

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
