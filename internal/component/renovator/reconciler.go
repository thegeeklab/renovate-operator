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
	scheme   *runtime.Scheme
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
		scheme:   scheme,
		req:      ctrl.Request{NamespacedName: client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}},
		instance: instance,
	}, nil
}

// Reconcile runs the Renovator component reconciliation pipeline. Status
// management (conditions, events and the status patch) is owned by the
// top-level controller; this function only mutates spec-driven children, the
// operation annotation and returns the aggregate result.
func (r *Reconciler) Reconcile(ctx context.Context) (*ctrl.Result, error) {
	results := &reconciler.Results{}

	reconcileFuncs := []func(context.Context) (*ctrl.Result, error){
		r.reconcileRenovateConfig,
		r.reconcileRenovateConfigMap,
		r.reconcileDiscovery,
		r.reconcileRunner,
	}

	var reconcileErr error

	for _, f := range reconcileFuncs {
		res, err := f(ctx)
		if err != nil {
			reconcileErr = err

			break
		}

		results.Collect(res)
	}

	result := results.ToResult()

	// Cleanup operation annotation when the work driven by it is finished
	// (either successfully or because it failed terminally). Only patch when
	// the annotation is actually present to avoid no-op writes.
	if (reconcileErr != nil || result.RequeueAfter == 0) && HasRenovatorOperation(r.instance.Annotations) {
		patch := client.MergeFrom(r.instance.DeepCopy())
		r.instance.Annotations = RemoveRenovatorOperation(r.instance.Annotations)

		if err := r.Patch(ctx, r.instance, patch); err != nil {
			return result, fmt.Errorf("remove operation annotation: %w", err)
		}
	}

	return result, reconcileErr
}
