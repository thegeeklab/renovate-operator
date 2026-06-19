package runner

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/component/runner"
	"github.com/thegeeklab/renovate-operator/internal/controller"
	"github.com/thegeeklab/renovate-operator/internal/frontend"
	batchv1 "k8s.io/api/batch/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
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
	Scheme        *runtime.Scheme
	Broker        *frontend.SSEBroker
	EventRecorder events.EventRecorder
}

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods/log,verbs=get;list;watch

// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=runners,verbs=get;list;watch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=runners/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos,verbs=get;list;watch

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

	original := rr.DeepCopy()

	outcome := r.reconcile(ctx, rr)
	controller.FinalizeStatus(ctx, r.Client, r.EventRecorder, original, rr, outcome,
		controller.FinalizeStatusOptions{SuccessMessage: "Runner reconciled successfully"})

	return controller.HandleReconcileResult(outcome.Result, outcome.Err)
}

// reconcile runs the Runner reconciliation pipeline.
func (r *Reconciler) reconcile(
	ctx context.Context, rr *renovatev1beta1.Runner,
) controller.Outcome {
	rcKey, err := r.resolveRenovateConfig(ctx, rr.Namespace, rr)
	if err != nil {
		controller.MarkNotReady(rr, renovatev1beta1.ReasonConfigResolutionFailed, err.Error())

		return controller.Outcome{Err: err}
	}

	rc := &renovatev1beta1.RenovateConfig{}
	if err := r.Get(ctx, rcKey, rc); err != nil {
		if api_errors.IsNotFound(err) {
			controller.MarkNotReady(rr, renovatev1beta1.ReasonConfigNotFound,
				controller.ErrRenovateConfigNotFound.Error())

			return controller.Outcome{Result: &ctrl.Result{}, Terminal: true}
		}

		return controller.Outcome{Err: err}
	}

	componentReconciler, err := runner.NewReconciler(r.Client, r.Scheme, r.Broker, rr, rc)
	if err != nil {
		return controller.Outcome{Err: err}
	}

	res, err := componentReconciler.Reconcile(ctx)

	return controller.Outcome{Result: res, Err: err}
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.EventRecorder = mgr.GetEventRecorder(ControllerName)

	const configRefIndexKey = ".spec.configRef"

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &renovatev1beta1.Runner{}, configRefIndexKey, runnerConfigRefIndexFunc,
	); err != nil {
		return err
	}

	renovateOperationPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldAnn := e.ObjectOld.GetAnnotations()
			newAnn := e.ObjectNew.GetAnnotations()

			return renovator.HasRenovatorOperationRenovate(newAnn) &&
				!renovator.HasRenovatorOperationRenovate(oldAnn)
		},
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}

	jobStatusPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldJob, ok1 := e.ObjectOld.(*batchv1.Job)

			newJob, ok2 := e.ObjectNew.(*batchv1.Job)
			if !ok1 || !ok2 {
				return false
			}

			return oldJob.Status.Succeeded != newJob.Status.Succeeded ||
				oldJob.Status.Failed != newJob.Status.Failed ||
				oldJob.Status.Active != newJob.Status.Active
		},
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.Runner{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			renovateOperationPredicate,
			jobStatusPredicate,
		)).
		Watches(
			&renovatev1beta1.GitRepo{},
			handler.EnqueueRequestsFromMapFunc(r.mapGitRepoToRunner),
			builder.WithPredicates(renovateOperationPredicate),
		).
		Watches(
			&renovatev1beta1.RenovateConfig{},
			handler.EnqueueRequestsFromMapFunc(r.mapConfigToRunner),
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

// mapGitRepoToRunner maps a GitRepo event to a Request for the Runner.
func (r *Reconciler) mapGitRepoToRunner(ctx context.Context, obj client.Object) []ctrl.Request {
	gitRepo, ok := obj.(*renovatev1beta1.GitRepo)
	if !ok {
		return nil
	}

	// Only enqueue requests for runners that match the renovate.thegeeklab.de/renovator label
	renovatorID, ok := gitRepo.Labels[renovatev1beta1.LabelRenovator]
	if !ok {
		return nil
	}

	runnerList := &renovatev1beta1.RunnerList{}
	if err := r.List(ctx, runnerList, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}

	var reqs []ctrl.Request

	for _, runner := range runnerList.Items {
		if runner.Labels != nil && runner.Labels[renovatev1beta1.LabelRenovator] == renovatorID {
			reqs = append(reqs, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      runner.Name,
					Namespace: runner.Namespace,
				},
			})
		}
	}

	return reqs
}

// mapConfigToRunner maps a RenovateConfig event to a Request for the Runner.
func (r *Reconciler) mapConfigToRunner(ctx context.Context, obj client.Object) []ctrl.Request {
	const configRefIndexKey = ".spec.configRef"

	runnerList := &renovatev1beta1.RunnerList{}
	if err := r.List(
		ctx, runnerList, client.InNamespace(obj.GetNamespace()), client.MatchingFields{configRefIndexKey: obj.GetName()},
	); err != nil {
		return nil
	}

	reqs := make([]ctrl.Request, len(runnerList.Items))
	for i := range runnerList.Items {
		reqs[i] = ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      runnerList.Items[i].Name,
				Namespace: runnerList.Items[i].Namespace,
			},
		}
	}

	return reqs
}

// resolveRenovateConfig resolves the RenovateConfig name from either .spec.configRef or renovatev1beta1.LabelRenovator.
func (r *Reconciler) resolveRenovateConfig(
	ctx context.Context, namespace string, rr *renovatev1beta1.Runner,
) (client.ObjectKey, error) {
	if rr.Spec.ConfigRef != "" {
		return client.ObjectKey{Namespace: namespace, Name: rr.Spec.ConfigRef}, nil
	}

	renovator, ok := rr.Labels[renovatev1beta1.LabelRenovator]
	if !ok {
		return client.ObjectKey{}, controller.ErrRenovateConfigNotFound
	}

	configList := &renovatev1beta1.RenovateConfigList{}
	if err := r.List(ctx, configList, client.InNamespace(namespace)); err != nil {
		return client.ObjectKey{}, err
	}

	for _, config := range configList.Items {
		if config.Labels != nil && config.Labels[renovatev1beta1.LabelRenovator] == renovator {
			return client.ObjectKey{Namespace: namespace, Name: config.Name}, nil
		}
	}

	return client.ObjectKey{}, controller.ErrRenovateConfigNotFound
}

// runnerConfigRefIndexFunc returns the config reference for indexing.
func runnerConfigRefIndexFunc(rawObj client.Object) []string {
	runner, ok := rawObj.(*renovatev1beta1.Runner)
	if !ok {
		return nil
	}

	if runner.Spec.ConfigRef == "" {
		return nil
	}

	return []string{runner.Spec.ConfigRef}
}
