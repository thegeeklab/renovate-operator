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
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
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

	rc := &renovatev1beta1.RenovateConfig{}
	rcNamespacedName := client.ObjectKey{Namespace: req.Namespace, Name: rd.Spec.ConfigRef}

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
		Owns(&batchv1.CronJob{}).
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
