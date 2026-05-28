package gitrepo

import (
	"context"
	"errors"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/gitrepo"
	"github.com/thegeeklab/renovate-operator/internal/controller"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const ControllerName = "gitrepo"

// Reconciler reconciles a GitRepo object.
type Reconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	ExternalURL   string
	EventRecorder events.EventRecorder
}

// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=gitrepos/finalizers,verbs=update
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovateconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

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

	original := gr.DeepCopy()

	outcome := r.reconcile(ctx, gr)
	controller.FinalizeStatus(ctx, r.Client, r.EventRecorder, original, gr, outcome,
		controller.FinalizeStatusOptions{SuccessMessage: "GitRepo reconciled successfully"})

	return controller.HandleReconcileResult(outcome.Result, outcome.Err)
}

// reconcile runs the GitRepo reconciliation pipeline.
func (r *Reconciler) reconcile(
	ctx context.Context, gr *renovatev1beta1.GitRepo,
) controller.Outcome {
	log := logf.FromContext(ctx)

	rcKey, err := r.resolveRenovateConfig(ctx, gr.Namespace, gr)
	if err != nil {
		if errors.Is(err, controller.ErrRenovateConfigNotFound) {
			log.V(1).Info("No RenovateConfig found for GitRepo, skipping",
				"object", client.ObjectKeyFromObject(gr))
			controller.MarkNotReady(gr, renovatev1beta1.ReasonConfigNotFound, err.Error())

			return controller.Outcome{Result: &ctrl.Result{}, Terminal: true}
		}

		return controller.Outcome{Err: err}
	}

	rc := &renovatev1beta1.RenovateConfig{}
	if err := r.Get(ctx, rcKey, rc); err != nil {
		if api_errors.IsNotFound(err) {
			controller.MarkNotReady(gr, renovatev1beta1.ReasonConfigNotFound,
				controller.ErrRenovateConfigNotFound.Error())

			return controller.Outcome{Result: &ctrl.Result{}, Terminal: true}
		}

		return controller.Outcome{Err: err}
	}

	componentReconciler, err := gitrepo.NewReconciler(r.Client, r.Scheme, r.ExternalURL, gr, rc)
	if err != nil {
		return controller.Outcome{Err: err}
	}

	res, err := componentReconciler.Reconcile(ctx)

	return controller.Outcome{Result: res, Err: err}
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.EventRecorder = mgr.GetEventRecorder(ControllerName)

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
		return client.ObjectKey{}, controller.ErrRenovateConfigNotFound
	}

	configList := &renovatev1beta1.RenovateConfigList{}
	if err := r.List(
		ctx, configList,
		client.InNamespace(namespace),
		client.MatchingLabels{renovatev1beta1.LabelRenovator: renovator},
	); err != nil {
		return client.ObjectKey{}, err
	}

	if len(configList.Items) == 0 {
		return client.ObjectKey{}, controller.ErrRenovateConfigNotFound
	}

	return client.ObjectKey{Namespace: namespace, Name: configList.Items[0].Name}, nil
}
