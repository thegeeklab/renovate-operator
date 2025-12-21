package runner

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/component/runner"
	"github.com/thegeeklab/renovate-operator/internal/controller"
	batchv1 "k8s.io/api/batch/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const ControllerName = "runner"

// Reconciler reconciles a Runner object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=runners,verbs=get;list;watch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=runners/status,verbs=get
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.V(1).Info("Reconciling object", "object", req.NamespacedName)

	rr := &renovatev1beta1.Runner{}

	if err := r.Get(ctx, req.NamespacedName, rr); err != nil {
		if api_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	rc := &renovatev1beta1.RenovateConfig{}

	if err := r.Get(ctx, req.NamespacedName, rc); err != nil {
		if api_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	runner, err := runner.NewReconciler(ctx, r.Client, r.Scheme, rr, rc)
	if err != nil {
		return ctrl.Result{}, err
	}

	if res, err := runner.Reconcile(ctx); err != nil {
		return controller.HandleReconcileResult(res, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.Runner{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					r, ok := e.ObjectNew.(*renovatev1beta1.Runner)
					if !ok {
						return false
					}

					or, ok := e.ObjectOld.(*renovatev1beta1.Runner)
					if !ok {
						return false
					}

					return (renovator.HasRenovatorOperationRenovate(r.Annotations) &&
						!renovator.HasRenovatorOperationRenovate(or.Annotations))
				},
				CreateFunc:  func(_ event.CreateEvent) bool { return true },
				DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
				GenericFunc: func(_ event.GenericEvent) bool { return false },
			},
		)).
		Watches(&renovatev1beta1.RenovateConfig{},
			handler.EnqueueRequestForOwner(
				r.Scheme,
				mgr.GetRESTMapper(),
				&renovatev1beta1.Runner{},
			),
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&renovatev1beta1.GitRepo{},
			handler.EnqueueRequestForOwner(
				r.Scheme,
				mgr.GetRESTMapper(),
				&renovatev1beta1.Runner{},
			),
			builder.WithPredicates(predicate.Or(
				predicate.GenerationChangedPredicate{},
				predicate.Funcs{
					UpdateFunc: func(e event.UpdateEvent) bool {
						r, ok := e.ObjectNew.(*renovatev1beta1.GitRepo)
						if !ok {
							return false
						}

						or, ok := e.ObjectOld.(*renovatev1beta1.GitRepo)
						if !ok {
							return false
						}

						return (renovator.HasRenovatorOperationRenovate(r.Annotations) &&
							!renovator.HasRenovatorOperationRenovate(or.Annotations))
					},
					CreateFunc:  func(_ event.CreateEvent) bool { return true },
					DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
					GenericFunc: func(_ event.GenericEvent) bool { return false },
				},
			))).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Named(ControllerName).
		Complete(r)
}
