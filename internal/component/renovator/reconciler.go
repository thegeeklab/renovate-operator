package renovator

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
	Scheme   *runtime.Scheme
	req      ctrl.Request
	instance *renovatev1beta1.Renovator
}

func NewReconciler(
	_ context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	instance *renovatev1beta1.Renovator,
) (*Reconciler, error) {
	return &Reconciler{
		Client:   c,
		Scheme:   scheme,
		req:      ctrl.Request{NamespacedName: client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}},
		instance: instance,
	}, nil
}

func (r *Reconciler) Reconcile(ctx context.Context) (*ctrl.Result, error) {
	results := &reconciler.Results{}

	// Define the reconciliation order
	reconcileFuncs := []func(context.Context) (*ctrl.Result, error){
		r.reconcileRenovateConfig,
		r.reconcileDiscovery,
		r.reconcileRunner,
	}

	// Execute each reconciliation step
	for _, reconcileFunc := range reconcileFuncs {
		res, err := reconcileFunc(ctx)
		if err != nil {
			return &ctrl.Result{Requeue: true}, fmt.Errorf("reconciliation failed: %w", err)
		}

		results.Collect(res)
	}

	// Remove discovery annotation
	r.instance.Annotations = RemoveRenovatorOperation(r.instance.Annotations)
	if err := r.Update(ctx, r.instance); err != nil {
		return &ctrl.Result{}, err
	}

	return results.ToResult(), nil
}
