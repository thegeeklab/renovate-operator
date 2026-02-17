package discovery

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/discovery"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/controller"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

const ControllerName = "discovery"

// Reconciler reconciles a Renovator object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//nolint:lll
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=discoveries,verbs=get;list;watch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=discoveries/status,verbs=get
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.V(1).Info("Reconciling object", "object", req.NamespacedName)

	rd := &renovatev1beta1.Discovery{}
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

	discovery, err := discovery.NewReconciler(ctx, r.Client, r.Scheme, rd, rc)
	if err != nil {
		return ctrl.Result{}, err
	}

	res, err := discovery.Reconcile(ctx)
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
		&renovatev1beta1.Discovery{},
		configRefIndexKey,
		discoveryConfigRefIndexFn,
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.Discovery{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldAnn := e.ObjectOld.GetAnnotations()
					newAnn := e.ObjectNew.GetAnnotations()

					return renovator.HasRenovatorOperationDiscover(newAnn) &&
						!renovator.HasRenovatorOperationDiscover(oldAnn)
				},
				CreateFunc:  func(_ event.CreateEvent) bool { return true },
				DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
				GenericFunc: func(_ event.GenericEvent) bool { return false },
			},
		)).
		Watches(&renovatev1beta1.RenovateConfig{},
			handler.EnqueueRequestsFromMapFunc(r.mapConfigToDiscovery),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return predicate.GenerationChangedPredicate{}.Update(e)
				},
				CreateFunc:  func(_ event.CreateEvent) bool { return true },
				DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
				GenericFunc: func(_ event.GenericEvent) bool { return false },
			}),
		).
		Owns(&renovatev1beta1.GitRepo{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&batchv1.Job{}).
		Named(ControllerName).
		Complete(r)
}

// mapConfigToDiscovery maps a RenovateConfig event to a Request for the Discovery.
func (r *Reconciler) mapConfigToDiscovery(ctx context.Context, obj client.Object) []ctrl.Request {
	const configRefIndexKey = ".spec.configRef"

	discoveryList := &renovatev1beta1.DiscoveryList{}
	if err := r.List(
		ctx,
		discoveryList,
		client.InNamespace(obj.GetNamespace()),
		client.MatchingFields{configRefIndexKey: obj.GetName()},
	); err != nil {
		return nil
	}

	reqs := make([]ctrl.Request, len(discoveryList.Items))
	for i := range discoveryList.Items {
		reqs[i] = ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      discoveryList.Items[i].Name,
				Namespace: discoveryList.Items[i].Namespace,
			},
		}
	}

	return reqs
}

// resolveRenovateConfig resolves the RenovateConfig name from either .spec.configRef or renovatev1beta1.RenovatorLabel.
func (r *Reconciler) resolveRenovateConfig(
	ctx context.Context,
	namespace string,
	rd *renovatev1beta1.Discovery,
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

// discoveryConfigRefIndexFn returns the config reference for indexing.
func discoveryConfigRefIndexFn(rawObj client.Object) []string {
	discovery, ok := rawObj.(*renovatev1beta1.Discovery)
	if !ok {
		return nil
	}

	if discovery.Spec.ConfigRef == "" {
		return nil
	}

	return []string{discovery.Spec.ConfigRef}
}
