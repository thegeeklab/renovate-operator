package cronjob

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteOwnedJobs deletes all Jobs owned by the specified CronJob.
func DeleteOwnedJobs(ctx context.Context, c client.Client, cronJob *batchv1.CronJob) error {
	// List all Jobs in the namespace
	jobList := &batchv1.JobList{}
	if err := c.List(ctx, jobList, client.InNamespace(cronJob.Namespace)); err != nil {
		return err
	}

	// Filter Jobs owned by the CronJob
	var jobsToDelete []batchv1.Job

	for _, job := range jobList.Items {
		if metav1.IsControlledBy(&job, cronJob) {
			jobsToDelete = append(jobsToDelete, job)
		}
	}

	// Delete the filtered Jobs
	for _, job := range jobsToDelete {
		if err := c.Delete(ctx, &job); err != nil {
			return err
		}
	}

	return nil
}
