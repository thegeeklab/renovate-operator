package gitrepo

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/provider"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	var reconcileErr error

	for _, reconcileFunc := range reconcileFuncs {
		res, err := reconcileFunc(ctx)
		if err != nil {
			reconcileErr = err

			break
		}

		results.Collect(res)
	}

	if statusErr := r.recordStatus(ctx, reconcileErr); statusErr != nil {
		ctrl.LoggerFrom(ctx).Error(statusErr, "failed to update status")
	}

	return results.ToResult(), reconcileErr
}

func (r *Reconciler) recordStatus(ctx context.Context, reconcileErr error) error {
	if reconcileErr != nil {
		r.instance.SetCondition(
			renovatev1beta1.ConditionReconciling,
			metav1.ConditionTrue,
			renovatev1beta1.ReasonReconcileError,
			reconcileErr.Error(),
		)
		r.instance.SetCondition(
			renovatev1beta1.ConditionReady,
			metav1.ConditionFalse,
			renovatev1beta1.ReasonReconcileError,
			reconcileErr.Error(),
		)
	} else {
		r.instance.SetCondition(
			renovatev1beta1.ConditionReady,
			metav1.ConditionTrue,
			renovatev1beta1.ReasonReconcileSuccess,
			"GitRepo reconciled successfully",
		)
		r.instance.RemoveCondition(renovatev1beta1.ConditionReconciling)
	}

	return r.Client.Status().Update(ctx, r.instance)
}
