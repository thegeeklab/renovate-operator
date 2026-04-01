package gitrepo

import (
	"context"
	"errors"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/gitrepo"
	"github.com/thegeeklab/renovate-operator/internal/controller"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const ControllerName = "gitrepo"

// Reconciler reconciles a GitRepo object.
type Reconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	ExternalURL string
}

// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos/finalizers,verbs=update
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovateconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.V(1).Info("Reconciling object", "object", req.NamespacedName)

	gr := &renovatev1beta1.GitRepo{}
	if err := r.Get(ctx, req.NamespacedName, gr); err != nil {
		if api_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	rcNamespacedName, err := r.resolveRenovateConfig(ctx, req.Namespace, gr)
	if err != nil {
		if errors.Is(err, controller.ErrNoRenovateConfigFound) {
			log.V(1).Info("No RenovateConfig found for GitRepo, skipping", "object", req.NamespacedName)

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	rc := &renovatev1beta1.RenovateConfig{}
	if err := r.Get(ctx, rcNamespacedName, rc); err != nil {
		if api_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	gitrepoReconciler, err := gitrepo.NewReconciler(r.Client, r.Scheme, r.ExternalURL, gr, rc)
	if err != nil {
		return ctrl.Result{}, err
	}

	res, err := gitrepoReconciler.Reconcile(ctx)
	if err != nil {
		return controller.HandleReconcileResult(res, err)
	}

	return *res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.GitRepo{}).
		Named(ControllerName).
		Complete(r)
}

// resolveRenovateConfig resolves the RenovateConfig name via the renovatev1beta1.LabelRenovator.
func (r *Reconciler) resolveRenovateConfig(
	ctx context.Context, namespace string, gr *renovatev1beta1.GitRepo,
) (client.ObjectKey, error) {
	renovator, ok := gr.Labels[renovatev1beta1.LabelRenovator]
	if !ok {
		return client.ObjectKey{}, controller.ErrNoRenovateConfigFound
	}

	configList := &renovatev1beta1.RenovateConfigList{}
	if err := r.List(ctx, configList,
		client.InNamespace(namespace),
		client.MatchingLabels{renovatev1beta1.LabelRenovator: renovator},
	); err != nil {
		return client.ObjectKey{}, err
	}

	if len(configList.Items) == 0 {
		return client.ObjectKey{}, controller.ErrNoRenovateConfigFound
	}

	return client.ObjectKey{Namespace: namespace, Name: configList.Items[0].Name}, nil
}
