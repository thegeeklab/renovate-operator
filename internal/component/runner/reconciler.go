package runner

import (
	"context"
	"errors"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/frontend"
	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrMaxRepoCount = errors.New("max repo count reached")

type Reconciler struct {
	client.Client
	scheme    *runtime.Scheme
	scheduler *scheduler.Manager
	broker    *frontend.SSEBroker
	req       ctrl.Request
	instance  *renovatev1beta1.Runner
	renovate  *renovatev1beta1.RenovateConfig
}

type JobData struct {
	Repositories []string `json:"repositories"`
}

func NewReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	broker *frontend.SSEBroker,
	instance *renovatev1beta1.Runner,
	renovate *renovatev1beta1.RenovateConfig,
) (*Reconciler, error) {
	return &Reconciler{
		Client:    c,
		scheme:    scheme,
		scheduler: scheduler.NewManager(c, scheme, clock.RealClock{}),
		req:       ctrl.Request{NamespacedName: client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}},
		instance:  instance,
		renovate:  renovate,
		broker:    broker,
	}, nil
}

func (r *Reconciler) Reconcile(ctx context.Context) (*ctrl.Result, error) {
	if r.broker != nil {
		r.broker.Broadcast("job-updated", "refresh")
	}

	results := &reconciler.Results{}

	reconcileFuncs := []func(context.Context) (*ctrl.Result, error){
		r.reconcileJob,
	}

	for _, reconcileFunc := range reconcileFuncs {
		result, err := reconcileFunc(ctx)
		if err != nil {
			return result, err
		}

		results.Collect(result)
	}

	return results.ToResult(), nil
}
