package gitrepo

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/provider"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	scheme          *runtime.Scheme
	req             ctrl.Request
	externalURL     string
	instance        *renovatev1beta1.GitRepo
	renovate        *renovatev1beta1.RenovateConfig
	providerFactory provider.ProviderFactory
}

func NewReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	externalURL string,
	instance *renovatev1beta1.GitRepo,
	renovate *renovatev1beta1.RenovateConfig,
) (*Reconciler, error) {
	return &Reconciler{
		Client:          c,
		scheme:          scheme,
		externalURL:     externalURL,
		req:             ctrl.Request{NamespacedName: client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}},
		instance:        instance,
		renovate:        renovate,
		providerFactory: provider.DefaultProviderFactory,
	}, nil
}

// Reconcile runs the GitRepo component reconciliation pipeline. Status
// management (conditions, events and the status patch) is owned by the
// top-level controller; this function only mutates spec-driven children and
// returns the aggregate result.
func (r *Reconciler) Reconcile(ctx context.Context) (*ctrl.Result, error) {
	results := &reconciler.Results{}

	var reconcileFuncs []func(context.Context) (*ctrl.Result, error)

	if r.instance.DeletionTimestamp.IsZero() {
		reconcileFuncs = []func(context.Context) (*ctrl.Result, error){
			r.reconcileGitRepo,
			r.reconcileWebhookSecret,
			r.reconcileWebhook,
		}
	} else {
		reconcileFuncs = []func(context.Context) (*ctrl.Result, error){
			r.reconcileWebhook,
			r.reconcileGitRepo,
		}
	}

	for _, reconcileFunc := range reconcileFuncs {
		res, err := reconcileFunc(ctx)
		if err != nil {
			return results.ToResult(), err
		}

		results.Collect(res)
	}

	return results.ToResult(), nil
}
