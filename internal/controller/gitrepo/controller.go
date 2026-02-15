package gitrepo

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/gitrepo"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/controller"
	batchv1 "k8s.io/api/batch/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const ControllerName = "gitrepo"

// Reconciler reconciles a Renovator object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

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

	rd := &renovatev1beta1.GitRepo{}
	if err := r.Get(ctx, req.NamespacedName, rd); err != nil {
		if api_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	rcNamespacedName, err := r.resolveRenovateConfig(ctx, req.Namespace, rd)
	if err != nil {
		return ctrl.Result{}, err
	}

	rc := &renovatev1beta1.RenovateConfig{}
	if err := r.Get(ctx, rcNamespacedName, rc); err != nil {
		if api_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	gitrepo, err := gitrepo.NewReconciler(ctx, r.Client, r.Scheme, rd, rc)
	if err != nil {
		return ctrl.Result{}, err
	}

	res, err := gitrepo.Reconcile(ctx)
	if err != nil {
		return controller.HandleReconcileResult(res, err)
	}

	return *res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	const configRefIndexKey = ".spec.configRef"

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&renovatev1beta1.GitRepo{},
		configRefIndexKey,
		gitrepoConfigRefIndexFn,
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.GitRepo{}).
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
		Owns(&batchv1.Job{}).
		Named(ControllerName).
		Complete(r)
}

// resolveRenovateConfig resolves the RenovateConfig name from either .spec.configRef or renovatev1beta1.RenovatorLabel.
func (r *Reconciler) resolveRenovateConfig(
	ctx context.Context,
	namespace string,
	rd *renovatev1beta1.GitRepo,
) (client.ObjectKey, error) {
	if rd.Spec.ConfigRef != "" {
		return client.ObjectKey{Namespace: namespace, Name: rd.Spec.ConfigRef}, nil
	}

	renovator, ok := rd.Labels[renovatev1beta1.RenovatorLabel]
	if !ok {
		return client.ObjectKey{}, controller.ErrNoRenovateConfigFound
	}

	configList := &renovatev1beta1.RenovateConfigList{}
	if err := r.List(ctx, configList, client.InNamespace(namespace)); err != nil {
		return client.ObjectKey{}, err
	}

	for _, config := range configList.Items {
		if config.Labels != nil && config.Labels[renovatev1beta1.RenovatorLabel] == renovator {
			return client.ObjectKey{Namespace: namespace, Name: config.Name}, nil
		}
	}

	return client.ObjectKey{}, controller.ErrNoRenovateConfigFound
}

// gitrepoConfigRefIndexFn returns the config reference for indexing.
func gitrepoConfigRefIndexFn(rawObj client.Object) []string {
	gitrepo, ok := rawObj.(*renovatev1beta1.GitRepo)
	if !ok {
		return nil
	}

	if gitrepo.Spec.ConfigRef == "" {
		return nil
	}

	return []string{gitrepo.Spec.ConfigRef}
}
