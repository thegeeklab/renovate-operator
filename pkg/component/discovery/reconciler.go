package discovery

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Req      ctrl.Request
	Instance *renovatev1beta1.Renovator
}

func (r *Reconciler) Reconcile(ctx context.Context, res *renovatev1beta1.Renovator) (*ctrl.Result, error) {
	results := &reconciler.Results{}

	reconcileFuncs := []func(context.Context) (*ctrl.Result, error){
		r.reconcileRole,
		r.reconcileRoleBinding,
		r.reconcileServiceAccount,
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
