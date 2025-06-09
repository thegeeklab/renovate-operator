package runner

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/reconciler"
	"github.com/thegeeklab/renovate-operator/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type runnerReconciler struct {
	*reconciler.GenericReconciler
	instance *renovatev1beta1.Renovator
	batches  []util.Batch
}

func Reconcile(
	ctx context.Context,
	kubeClient client.Client,
	scheme *runtime.Scheme,
	req ctrl.Request,
	instance *renovatev1beta1.Renovator,
) (*ctrl.Result, error) {
	r := &runnerReconciler{
		GenericReconciler: &reconciler.GenericReconciler{
			KubeClient: kubeClient,
			Scheme:     scheme,
			Req:        req,
		},
		instance: instance,
	}

	batches, err := r.CreateBatches(ctx)
	if err != nil {
		return &ctrl.Result{}, err
	}

	r.batches = batches

	results := &reconciler.Results{}
	var res *ctrl.Result

	// Reconcile CronJob for scheduled runs
	res, err = r.reconcileCronJob(ctx)
	if err != nil {
		return res, err
	}

	results.Collect(res)

	res, err = r.reconcileConfigMap(ctx)
	if err != nil {
		return res, err
	}

	results.Collect(res)

	// Only reconcile RenovatorJobs if no schedule is set (immediate mode)
	if r.instance.Spec.Schedule == "" {
		res, err = r.reconcileRenovatorJobs(ctx)
		if err != nil {
			return res, err
		}

		results.Collect(res)
	}

	return results.ToResult(), nil
}
