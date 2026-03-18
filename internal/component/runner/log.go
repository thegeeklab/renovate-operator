package runner

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	batchv1 "k8s.io/api/batch/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const maxLogsPerReconcile = 5

// reconcileLogs archives logs for completed runner jobs to the persistent store.
func (r *Reconciler) reconcileLogs(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	runnerLabels := RunnerLabels(r.req)

	if val, ok := r.instance.Labels[renovatev1beta1.LabelRenovator]; ok {
		runnerLabels[renovatev1beta1.LabelRenovator] = val
	}

	var jobList batchv1.JobList

	err := r.List(ctx, &jobList, client.InNamespace(r.instance.Namespace), client.MatchingLabels(runnerLabels))
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to list runner jobs for log collection: %w", err)
	}

	processedCount := 0
	requeueNeeded := false

	for _, job := range jobList.Items {
		if !scheduler.IsJobFinished(&job) {
			continue
		}

		if job.Annotations[renovatev1beta1.LabelLogsCollected] == renovatev1beta1.ValueTrue {
			continue
		}

		if processedCount >= maxLogsPerReconcile {
			requeueNeeded = true

			break
		}

		log.V(1).Info("Archiving logs for finished runner job", "job", job.Name)

		err := r.logManager.ArchiveJob(ctx, job.Namespace, job.Name, "runner", r.instance.Name, "renovate")
		if err != nil {
			log.Error(err, "Failed to archive logs for runner job", "job", job.Name)

			continue
		}

		patch := client.MergeFrom(job.DeepCopy())
		if job.Annotations == nil {
			job.Annotations = make(map[string]string)
		}

		job.Annotations[renovatev1beta1.LabelLogsCollected] = renovatev1beta1.ValueTrue

		if err := r.Patch(ctx, &job, patch); err != nil {
			log.Error(err, "Failed to patch job annotation after log collection", "job", job.Name)
		}

		processedCount++
	}

	if requeueNeeded {
		log.Info("Log batch limit reached, requeuing to process remaining jobs")

		return &ctrl.Result{Requeue: true}, nil
	}

	return &ctrl.Result{}, nil
}
