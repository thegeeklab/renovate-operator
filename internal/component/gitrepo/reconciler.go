package gitrepo

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/provider"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	scheme          *runtime.Scheme
	eventRecorder   events.EventRecorder
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
	recorder events.EventRecorder,
	instance *renovatev1beta1.GitRepo,
	renovate *renovatev1beta1.RenovateConfig,
) (*Reconciler, error) {
	return &Reconciler{
		Client:          c,
		scheme:          scheme,
		externalURL:     externalURL,
		eventRecorder:   recorder,
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

	conditionType := renovatev1beta1.ConditionReady
	reason := renovatev1beta1.ReasonReconcileSuccess
	message := "GitRepo reconciled successfully"
	eventType := renovatev1beta1.EventTypeNormal

	if reconcileErr != nil {
		conditionType = renovatev1beta1.ConditionReconcileError
		reason = renovatev1beta1.ReasonReconcileError
		message = reconcileErr.Error()
		eventType = renovatev1beta1.EventTypeWarning
	}

	r.instance.SetCondition(conditionType, "True", reason, message)
	r.eventRecorder.Eventf(r.instance, nil, eventType, reason, reason, "%s", message)

	if err := r.Client.Status().Update(ctx, r.instance); err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	return results.ToResult(), reconcileErr
}
