package runner

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type runnerReconciler struct {
	*reconciler.GenericReconciler
	instance *renovatev1beta1.Renovator
}

func Reconcile(
	ctx context.Context,
	client client.Client,
	scheme *runtime.Scheme,
	req ctrl.Request,
	instance *renovatev1beta1.Renovator,
) (*ctrl.Result, error) {
	r := &runnerReconciler{
		GenericReconciler: &reconciler.GenericReconciler{
			Client: client,
			Scheme: scheme,
			Req:    req,
		},
		instance: instance,
	}
	// repos := r.instance.Status.Repositories

	// var batches []Batch

	// switch r.instance.Spec.Runner.Strategy {
	// case renovatev1beta1.RunnerStrategy_BATCH:
	// 	limit := r.instance.Spec.Runner.BatchSize
	// 	for i := 0; i < len(repos); i += limit {
	// 		batch := repos[i:min(i+limit, len(repos))]
	// 		batches = append(batches, Batch{Repositories: batch})
	// 	}
	// case renovatev1beta1.RunnerStrategy_NONE:
	// default:
	// 	batches = append(batches, Batch{Repositories: repos})
	// }

	// r.batches = batches

	results := &reconciler.Results{}

	res, err := r.reconcileConfigMap(ctx)
	if err != nil {
		return res, err
	}

	results.Collect(res)

	return results.ToResult(), nil
}
