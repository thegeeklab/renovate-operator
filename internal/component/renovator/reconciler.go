package renovator

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	if statusErr := r.recordStatus(ctx, reconcileErr); statusErr != nil {
		ctrl.LoggerFrom(ctx).Error(statusErr, "failed to update status")
	}

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

func (r *Reconciler) recordStatus(ctx context.Context, reconcileErr error) error {
	if reconcileErr != nil {
		r.instance.SetCondition(
			renovatev1beta1.ConditionReconciling,
			metav1.ConditionTrue,
			renovatev1beta1.ReasonReconcileError,
			reconcileErr.Error(),
		)
		r.instance.SetCondition(
			renovatev1beta1.ConditionReady,
			metav1.ConditionFalse,
			renovatev1beta1.ReasonReconcileError,
			reconcileErr.Error(),
		)
	} else {
		r.instance.SetCondition(
			renovatev1beta1.ConditionReady,
			metav1.ConditionTrue,
			renovatev1beta1.ReasonReconcileSuccess,
			"Renovator reconciled successfully",
		)
		r.instance.RemoveCondition(renovatev1beta1.ConditionReconciling)
	}

	return r.Client.Status().Update(ctx, r.instance)
}
