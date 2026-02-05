package scheduler

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/component/scheduler"
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

const ControllerName = "scheduler"

// Reconciler reconciles a Scheduler object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=schedulers,verbs=get;list;watch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=schedulers/status,verbs=get
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.V(1).Info("Reconciling object", "object", req.NamespacedName)

	rr := &renovatev1beta1.Scheduler{}

	if err := r.Get(ctx, req.NamespacedName, rr); err != nil {
		if api_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	rc := &renovatev1beta1.RenovateConfig{}
	rcNamespacedName := client.ObjectKey{Namespace: req.Namespace, Name: rr.Spec.ConfigRef}

	if err := r.Get(ctx, rcNamespacedName, rc); err != nil {
		if api_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	scheduler, err := scheduler.NewReconciler(ctx, r.Client, r.Scheme, rr, rc)
	if err != nil {
		return ctrl.Result{}, err
	}

	if res, err := scheduler.Reconcile(ctx); err != nil {
		return controller.HandleReconcileResult(res, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	const configRefIndexKey = ".spec.configRef"

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&renovatev1beta1.Scheduler{},
		configRefIndexKey,
		schedulerConfigRefIndexFunc,
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.Scheduler{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldAnn := e.ObjectOld.GetAnnotations()
					newAnn := e.ObjectNew.GetAnnotations()

					return renovator.HasRenovatorOperationRenovate(newAnn) &&
						!renovator.HasRenovatorOperationRenovate(oldAnn)
				},
				CreateFunc:  func(_ event.CreateEvent) bool { return true },
				DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
				GenericFunc: func(_ event.GenericEvent) bool { return false },
			},
		)).
		Watches(&renovatev1beta1.GitRepo{},
			handler.EnqueueRequestsFromMapFunc(r.mapGitRepoToScheduler),
			builder.WithPredicates(predicate.Or(
				predicate.GenerationChangedPredicate{},
				predicate.Funcs{
					UpdateFunc: func(e event.UpdateEvent) bool {
						oldAnn := e.ObjectOld.GetAnnotations()
						newAnn := e.ObjectNew.GetAnnotations()

						return renovator.HasRenovatorOperationRenovate(newAnn) &&
							!renovator.HasRenovatorOperationRenovate(oldAnn)
					},
					CreateFunc:  func(_ event.CreateEvent) bool { return false },
					DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
					GenericFunc: func(_ event.GenericEvent) bool { return false },
				}),
			),
		).
		Watches(&renovatev1beta1.RenovateConfig{},
			handler.EnqueueRequestsFromMapFunc(r.mapConfigToScheduler),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return predicate.GenerationChangedPredicate{}.Update(e)
				},
				CreateFunc:  func(_ event.CreateEvent) bool { return true },
				DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
				GenericFunc: func(_ event.GenericEvent) bool { return false },
			}),
		).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Named(ControllerName).
		Complete(r)
}

// mapGitRepoToScheduler maps a GitRepo event to a Request for the Scheduler.
func (r *Reconciler) mapGitRepoToScheduler(ctx context.Context, obj client.Object) []ctrl.Request {
	gitRepo, ok := obj.(*renovatev1beta1.GitRepo)
	if !ok {
		return nil
	}

	// Only enqueue requests for schedulers that match the renovate.thegeeklab.de/renovator label
	if gitRepo.Labels == nil {
		return nil
	}

	renovatorName, ok := gitRepo.Labels[renovatev1beta1.RenovatorLabel]
	if !ok {
		return nil
	}

	schedulerList := &renovatev1beta1.SchedulerList{}
	if err := r.List(ctx, schedulerList, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}

	var reqs []ctrl.Request
	for _, scheduler := range schedulerList.Items {
		// Check if the scheduler's renovate.thegeeklab.de/renovator label matches the renovatorName
		if scheduler.Labels != nil && scheduler.Labels[renovatev1beta1.RenovatorLabel] == renovatorName {
			reqs = append(reqs, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      scheduler.Name,
					Namespace: scheduler.Namespace,
				},
			})
		}
	}

	return reqs
}

// mapConfigToScheduler maps a RenovateConfig event to a Request for the Scheduler.
func (r *Reconciler) mapConfigToScheduler(ctx context.Context, obj client.Object) []ctrl.Request {
	const configRefIndexKey = ".spec.configRef"

	schedulerList := &renovatev1beta1.SchedulerList{}
	if err := r.List(
		ctx,
		schedulerList,
		client.InNamespace(obj.GetNamespace()),
		client.MatchingFields{configRefIndexKey: obj.GetName()},
	); err != nil {
		return nil
	}

	reqs := make([]ctrl.Request, len(schedulerList.Items))
	for i := range schedulerList.Items {
		reqs[i] = ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      schedulerList.Items[i].Name,
				Namespace: schedulerList.Items[i].Namespace,
			},
		}
	}

	return reqs
}

// schedulerConfigRefIndexFunc returns the config reference for indexing.
func schedulerConfigRefIndexFunc(rawObj client.Object) []string {
	scheduler, ok := rawObj.(*renovatev1beta1.Scheduler)
	if !ok {
		return nil
	}

	if scheduler.Spec.ConfigRef == "" {
		return nil
	}

	return []string{scheduler.Spec.ConfigRef}
}
