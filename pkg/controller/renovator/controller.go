package renovator

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/component/discovery"
	"github.com/thegeeklab/renovate-operator/pkg/component/runner"
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

// Reconciler reconciles a Renovator object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//nolint:lll
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete;deletecollection
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

	discovery, err := discovery.NewReconciler(ctx, r.Client, r.Scheme, rr)
	if err != nil {
		return ctrl.Result{}, err
	}

	if res, err := discovery.Reconcile(ctx, rr); err != nil {
		return handleReconcileResult(res, err)
	}

	runner, err := runner.NewReconciler(ctx, r.Client, r.Scheme, rr)
	if err != nil {
		return ctrl.Result{}, err
	}

	if res, err := runner.Reconcile(ctx, rr); err != nil {
		return handleReconcileResult(res, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.Renovator{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, r.HasOperationAnnotation()),
		)).
		Watches(&renovatev1beta1.GitRepo{}, handler.EnqueueRequestForOwner(
			r.Scheme,
			mgr.GetRESTMapper(),
			&renovatev1beta1.Renovator{},
		), builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, r.HasOperationAnnotation()),
		)).
		Owns(&renovatev1beta1.GitRepo{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Named("renovator").
		Complete(r)
}

// HasOperationAnnotation returns a predicate which returns true when the object has an operation annotation.
func (r *Reconciler) HasOperationAnnotation() predicate.Funcs {
	hasOperationAnnotation := func(annotations map[string]string) bool {
		return annotations[renovatev1beta1.AnnotationOperation] != ""
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return hasOperationAnnotation(e.Object.GetAnnotations())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return !hasOperationAnnotation(e.ObjectOld.GetAnnotations()) && hasOperationAnnotation(e.ObjectNew.GetAnnotations())
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
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
