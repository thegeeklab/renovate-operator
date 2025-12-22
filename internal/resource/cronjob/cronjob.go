package cronjob

import (
	"context"
	"strings"
	"time"

	"github.com/thegeeklab/renovate-operator/pkg/util"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// RequeueDelay is the default delay when requeuing operations.
	RequeueDelay = time.Minute
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

// CheckActiveJobs checks if there are any active jobs with the given name pattern.
func CheckActiveJobs(ctx context.Context, c client.Client, namespace, jobName string) (bool, error) {
	existingJobs := &batchv1.JobList{}
	if err := c.List(ctx, existingJobs, client.InNamespace(namespace)); err != nil {
		return false, err
	}

	for _, job := range existingJobs.Items {
		if job.Name == jobName || strings.HasPrefix(job.Name, jobName+"-") {
			// Check if the job has active pods
			if job.Status.Active > 0 {
				return true, nil
			}

			// Check if the job is not yet completed and expects completions
			if util.PtrIsNonZero(job.Spec.Completions) && job.Status.Succeeded == 0 && job.Status.Failed == 0 {
				return true, nil
			}
		}
	}

	return false, nil
}
