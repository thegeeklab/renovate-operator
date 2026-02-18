package discovery

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	scheme   *runtime.Scheme
	req      ctrl.Request
	instance *renovatev1beta1.Discovery
	renovate *renovatev1beta1.RenovateConfig
}

func NewReconciler(
	_ context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	instance *renovatev1beta1.Discovery,
	renovate *renovatev1beta1.RenovateConfig,
) (*Reconciler, error) {
	return &Reconciler{
		Client:   c,
		scheme:   scheme,
		req:      ctrl.Request{NamespacedName: client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}},
		instance: instance,
		renovate: renovate,
	}, nil
}

func (r *Reconciler) Reconcile(ctx context.Context) (*ctrl.Result, error) {
	results := &reconciler.Results{}

	// Define the reconciliation order
	reconcileFuncs := []func(context.Context) (*ctrl.Result, error){
		r.reconcileRole,
		r.reconcileRoleBinding,
		r.reconcileServiceAccount,
		r.reconcileJob,
		r.reconcileGitRepos,
	}

	// Execute each reconciliation step
	for _, reconcileFunc := range reconcileFuncs {
		res, err := reconcileFunc(ctx)
		if err != nil {
			return &ctrl.Result{}, fmt.Errorf("reconciliation failed: %w", err)
		}

		results.Collect(res)
	}

	return results.ToResult(), nil
}
