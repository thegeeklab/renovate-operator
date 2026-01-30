package renovateconfig

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	renovateconfig "github.com/thegeeklab/renovate-operator/internal/component/renovateconfig"
	"github.com/thegeeklab/renovate-operator/internal/controller"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const ControllerName = "renovateconfig"

// Reconciler reconciles a RenovateConfig object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovateconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovateconfigs/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.V(1).Info("Reconciling object", "object", req.NamespacedName)

	rc := &renovatev1beta1.RenovateConfig{}

	if err := r.Get(ctx, req.NamespacedName, rc); err != nil {
		if api_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	renovateconfig, err := renovateconfig.NewReconciler(ctx, r.Client, r.Scheme, rc)
	if err != nil {
		return ctrl.Result{}, err
	}

	if res, err := renovateconfig.Reconcile(ctx); err != nil {
		return controller.HandleReconcileResult(res, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.RenovateConfig{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named(ControllerName).
		Complete(r)
}
