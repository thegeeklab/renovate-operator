package renovator

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
	"sigs.k8s.io/controller-runtime/pkg/handler"
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
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

//nolint:lll
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovators/finalizers,verbs=update
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos,verbs=get;list;watch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos/status,verbs=get
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=discoveries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=discoveries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=discoveries/finalizers,verbs=update
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovateconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovateconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovateconfigs/finalizers,verbs=update

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

	renovator, err := renovator.NewReconciler(ctx, r.Client, r.Scheme, rr)
	if err != nil {
		return ctrl.Result{}, err
	}

	if res, err := renovator.Reconcile(ctx); err != nil {
		return controller.HandleReconcileResult(res, err)
	}

	rc := &renovatev1beta1.RenovateConfig{}

	if err := r.Get(ctx, req.NamespacedName, rc); err != nil {
		if !api_errors.IsNotFound(err) {
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
		For(&renovatev1beta1.Renovator{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}),
		)).
		Watches(&renovatev1beta1.GitRepo{}, handler.EnqueueRequestForOwner(
			r.Scheme,
			mgr.GetRESTMapper(),
			&renovatev1beta1.Renovator{},
		), builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}),
		)).
		Owns(&renovatev1beta1.Discovery{}).
		Owns(&renovatev1beta1.RenovateConfig{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Named(ControllerName).
		Complete(r)
}
