package gitrepo

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/provider"
	"github.com/thegeeklab/renovate-operator/internal/provider/gitea"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const DummyWebhookURL = "http://renovate-webhook.renovate-system.svc.cluster.local/webhook"

type ProviderFactory func(
	ctx context.Context,
	c client.Client,
	instance *renovatev1beta1.GitRepo,
	renovate *renovatev1beta1.RenovateConfig,
) (provider.WebhookManager, error)

type Reconciler struct {
	client.Client
	scheme          *runtime.Scheme
	req             ctrl.Request
	instance        *renovatev1beta1.GitRepo
	renovate        *renovatev1beta1.RenovateConfig
	ProviderFactory ProviderFactory
}

func NewReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	instance *renovatev1beta1.GitRepo,
	renovate *renovatev1beta1.RenovateConfig,
) (*Reconciler, error) {
	return &Reconciler{
		Client:          c,
		scheme:          scheme,
		req:             ctrl.Request{NamespacedName: client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}},
		instance:        instance,
		renovate:        renovate,
		ProviderFactory: defaultProviderFactory,
	}, nil
}

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
			return &ctrl.Result{}, fmt.Errorf("reconciliation failed: %w", err)
		}

		results.Collect(res)
	}

	return results.ToResult(), nil
}

//nolint:ireturn
func defaultProviderFactory(
	ctx context.Context, c client.Client, instance *renovatev1beta1.GitRepo, renovate *renovatev1beta1.RenovateConfig,
) (provider.WebhookManager, error) {
	if renovate.Spec.Platform.Type != "gitea" {
		return nil, provider.ErrNotImplemented
	}

	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      renovate.Spec.Platform.Token.SecretKeyRef.Name,
	}

	if err := c.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("failed to fetch secret for provider token: %w", err)
	}

	token := string(secret.Data[renovate.Spec.Platform.Token.SecretKeyRef.Key])

	return gitea.NewProvider(renovate.Spec.Platform.Endpoint, token)
}
