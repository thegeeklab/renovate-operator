package discovery

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type discoveryReconciler struct {
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
	r := &discoveryReconciler{
		GenericReconciler: &reconciler.GenericReconciler{
			Client: client,
			Scheme: scheme,
			Req:    req,
		},
		instance: instance,
	}

	results := &reconciler.Results{}

	res, err := r.reconcileServiceAccount(ctx)
	if err != nil {
		return res, err
	}

	results.Collect(res)

	res, err = r.reconcileRole(ctx)
	if err != nil {
		return res, err
	}

	results.Collect(res)

	res, err = r.reconcileRoleBinding(ctx)
	if err != nil {
		return res, err
	}

	results.Collect(res)

	res, err = r.reconcileCronJob(ctx)
	if err != nil {
		return res, err
	}

	results.Collect(res)

	return results.ToResult(), nil
}
