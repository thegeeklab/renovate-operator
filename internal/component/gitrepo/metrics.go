package gitrepo

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileMetrics manages the FinalizerGitRepoMetrics finalizer and releases
// the per-repo Prometheus metric series when the GitRepo is deleted. It
// mirrors the pattern used by reconcileGitRepo for FinalizerGitRepoWebhook.
func (r *Reconciler) reconcileMetrics(ctx context.Context) (*ctrl.Result, error) {
	if !r.instance.DeletionTimestamp.IsZero() {
		return r.handleMetricsDeletion(ctx)
	}

	if !controllerutil.ContainsFinalizer(r.instance, renovatev1beta1.FinalizerGitRepoMetrics) {
		patch := client.MergeFrom(r.instance.DeepCopy())
		controllerutil.AddFinalizer(r.instance, renovatev1beta1.FinalizerGitRepoMetrics)

		if err := r.Patch(ctx, r.instance, patch); err != nil {
			return &ctrl.Result{}, err
		}
	}

	return &ctrl.Result{}, nil
}

// handleMetricsDeletion releases metrics and removes the finalizer when the GitRepo is being deleted.
func (r *Reconciler) handleMetricsDeletion(ctx context.Context) (*ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(r.instance, renovatev1beta1.FinalizerGitRepoMetrics) {
		return &ctrl.Result{}, nil
	}

	if r.metrics != nil {
		r.releaseMetricsForGitRepo(ctx)
	}

	patch := client.MergeFrom(r.instance.DeepCopy())
	controllerutil.RemoveFinalizer(r.instance, renovatev1beta1.FinalizerGitRepoMetrics)

	if err := r.Patch(ctx, r.instance, patch); err != nil && !api_errors.IsNotFound(err) {
		return &ctrl.Result{}, err
	}

	return &ctrl.Result{}, nil
}

// releaseMetricsForGitRepo enumerates Runner resources in the GitRepo's
// namespace and releases the per-runner metric series for the given GitRepo.
// A single GitRepo can be observed by multiple Runner instances (one per
// operator deployment), so all matching runner label combinations must be
// cleaned up to free the cardinality cap.
func (r *Reconciler) releaseMetricsForGitRepo(ctx context.Context) {
	renovatorLabel := r.instance.Labels[renovatev1beta1.LabelRenovator]

	runnerList := &renovatev1beta1.RunnerList{}
	if err := r.List(ctx, runnerList, client.InNamespace(r.instance.Namespace)); err != nil {
		return
	}

	for i := range runnerList.Items {
		r.metrics.DeleteGitRepo(r.instance.Namespace, renovatorLabel, runnerList.Items[i].Name, r.instance.Name)
	}
}
