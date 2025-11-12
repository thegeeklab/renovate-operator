package runner

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RunnerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Req      ctrl.Request
	Instance *renovatev1beta1.Renovator
	Batches  []Batch
}

type Batch struct {
	Repositories []string `json:"repositories"`
}

type Renovate struct {
	Onboarding    bool                         `json:"onboarding"`
	PrHourlyLimit int                          `json:"prHourlyLimit"`
	DryRun        renovatev1beta1.DryRun       `json:"dryRun"`
	Platform      renovatev1beta1.PlatformType `json:"platform"`
	Endpoint      string                       `json:"endpoint"`
	AddLabels     []string                     `json:"addLabels,omitempty"`
	Repositories  []string                     `json:"repositories"`
}

func (r *RunnerReconciler) Reconcile(ctx context.Context, res *renovatev1beta1.Renovator) (*ctrl.Result, error) {
	results := &reconciler.Results{}

	reconcileFuncs := []func(context.Context) (*ctrl.Result, error){
		r.reconcileConfigMap,
		r.reconcileCronJob,
	}

	for _, reconcileFunc := range reconcileFuncs {
		res, err := reconcileFunc(ctx)
		if err != nil {
			return res, err
		}

		results.Collect(res)
	}

	return results.ToResult(), nil
}
