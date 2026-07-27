package discovery

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileMetrics manages the shared metrics-cleanup finalizer and releases
// the per-discovery Prometheus metric series when the Discovery is deleted.
func (r *Reconciler) reconcileMetrics(ctx context.Context) (*ctrl.Result, error) {
	if !r.instance.DeletionTimestamp.IsZero() {
		return r.handleMetricsDeletion(ctx)
	}

	if !controllerutil.ContainsFinalizer(r.instance, renovatev1beta1.FinalizerMetricsCleanup) {
		patch := client.MergeFrom(r.instance.DeepCopy())
		controllerutil.AddFinalizer(r.instance, renovatev1beta1.FinalizerMetricsCleanup)

		if err := r.Patch(ctx, r.instance, patch); err != nil {
			return &ctrl.Result{}, err
		}
	}

	return &ctrl.Result{}, nil
}

// handleMetricsDeletion releases metrics and removes the finalizer when the Discovery is being deleted.
func (r *Reconciler) handleMetricsDeletion(ctx context.Context) (*ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(r.instance, renovatev1beta1.FinalizerMetricsCleanup) {
		return &ctrl.Result{}, nil
	}

	if r.metrics != nil {
		r.releaseMetricsForDiscovery()
	}

	patch := client.MergeFrom(r.instance.DeepCopy())
	controllerutil.RemoveFinalizer(r.instance, renovatev1beta1.FinalizerMetricsCleanup)

	if err := r.Patch(ctx, r.instance, patch); err != nil && !api_errors.IsNotFound(err) {
		return &ctrl.Result{}, err
	}

	return &ctrl.Result{}, nil
}

// releaseMetricsForDiscovery releases the per-discovery metric series.
func (r *Reconciler) releaseMetricsForDiscovery() {
	renovatorLabel := r.instance.Labels[renovatev1beta1.LabelRenovator]

	r.metrics.DeleteDiscovery(r.instance.Namespace, renovatorLabel, r.instance.Name)
}
