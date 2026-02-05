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
		r.reconcileRenovateConfigMap,
		r.reconcileDiscovery,
		r.reconcileScheduler,
	}

	// Execute each reconciliation step
	for _, reconcileFunc := range reconcileFuncs {
		res, err := reconcileFunc(ctx)
		if err != nil {
			// If reconciliation fails, remove the operation annotation to prevent retry loops
			r.instance.Annotations = RemoveRenovatorOperation(r.instance.Annotations)
			if updateErr := r.Update(ctx, r.instance); updateErr != nil {
				return &ctrl.Result{}, fmt.Errorf("reconciliation failed: %w: failed to remove operation annotation: %w",
					err, updateErr)
			}

			return &ctrl.Result{}, fmt.Errorf("reconciliation failed: %w", err)
		}

		results.Collect(res)
	}

	// Remove operation annotation only if reconciliation was successful (no requeue needed)
	result := results.ToResult()
	if result.RequeueAfter == 0 {
		r.instance.Annotations = RemoveRenovatorOperation(r.instance.Annotations)
		if err := r.Update(ctx, r.instance); err != nil {
			return &ctrl.Result{}, fmt.Errorf("failed to remove operation annotation: %w", err)
		}
	}

	return result, nil
}
