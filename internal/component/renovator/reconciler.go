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

	// Cleanup annotation if reconciliation failed or finished successfully
	// Only remove the annotation if it exists to avoid nil map issues
	if (reconcileErr != nil || result.RequeueAfter == 0) && HasRenovatorOperation(r.instance.Annotations) {
		patch := client.MergeFrom(r.instance.DeepCopy())
		r.instance.Annotations = RemoveRenovatorOperation(r.instance.Annotations)

		if err := r.Patch(ctx, r.instance, patch); err != nil {
			return &ctrl.Result{}, fmt.Errorf("failed to remove operation annotation: %w", err)
		}
	}

	return result, reconcileErr
}
