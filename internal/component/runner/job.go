package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/netresearch/go-cron"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ReconcileJob determines if a global run is due, processes GitRepo resources,
// and schedules the next run if necessary.
func (r *Reconciler) reconcileJob(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Evaluate if the scheduled run is due
	isCronDue, nextRun, err := r.evaluateSchedule()
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to evaluate schedule: %w", err)
	}

	// Check for manual trigger and suspension state
	manualRunnerTrigger := renovator.HasRenovatorOperationRenovate(r.instance.Annotations)
	isSuspended := r.instance.Spec.Suspend != nil && *r.instance.Spec.Suspend

	// Determine if a global trigger is active
	isGlobalTriggerActive := (isCronDue && !isSuspended) || manualRunnerTrigger

	if isSuspended && isCronDue && !manualRunnerTrigger {
		log.V(1).Info("Runner is suspended: suppressing scheduled run")
	}

	// Process all GitRepo resources
	triggeredAny, err := r.processGitRepos(ctx, isGlobalTriggerActive)
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to process GitRepos: %w", err)
	}

	// Update status and annotations if a global run was triggered
	if manualRunnerTrigger || (isCronDue && !isSuspended) {
		return r.updateStatusAfterRun(ctx)
	}

	// If only individual repos were triggered, return without rescheduling
	if triggeredAny && !isGlobalTriggerActive {
		return &ctrl.Result{}, nil
	}

	// Schedule the next run
	now := time.Now()
	if nextRun.After(now) {
		log.V(1).Info("Next runner execution scheduled", "time", nextRun, "wait", nextRun.Sub(now))

		return &ctrl.Result{RequeueAfter: nextRun.Sub(now)}, nil
	}

	return &ctrl.Result{}, nil
}

// processGitRepos processes each GitRepo and creates jobs if needed.
func (r *Reconciler) processGitRepos(ctx context.Context, isGlobalTriggerActive bool) (bool, error) {
	log := logf.FromContext(ctx)
	triggeredAny := false

	// List all GitRepos in the namespace
	gitRepos := &renovatev1beta1.GitRepoList{}
	if err := r.List(ctx, gitRepos, client.InNamespace(r.instance.Namespace)); err != nil {
		return false, fmt.Errorf("failed to list GitRepos: %w", err)
	}

	// Iterate over each GitRepo
	for _, repo := range gitRepos.Items {
		// Skip if neither global trigger nor repo annotation is present
		hasRepoAnnotation := renovator.HasRenovatorOperationRenovate(repo.Annotations)
		if !isGlobalTriggerActive && !hasRepoAnnotation {
			continue
		}

		triggeredAny = true

		// Check for active jobs (locking mechanism)
		repoLabels := map[string]string{
			renovatev1beta1.RenovatorLabel:   r.instance.Labels[renovatev1beta1.RenovatorLabel],
			"renovate.thegeeklab.de/gitrepo": repo.Name,
		}

		activeJobs, err := renovate.GetActiveJobs(ctx, r.Client, repo.Namespace, repoLabels)
		if err != nil {
			log.Error(err, "Failed to check active jobs", "repo", repo.Name)

			continue
		}

		if len(activeJobs) > 0 {
			log.V(1).Info("Active renovate job found: skipping", "repo", repo.Name)

			continue
		}

		// Create and execute the job
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: repo.Name + "-",
				Namespace:    repo.Namespace,
				Labels:       repoLabels,
			},
		}
		r.updateJob(job, &repo)

		// Create the job
		if err := k8s.Create(ctx, r.Client, job, r.instance, nil); err != nil {
			log.Error(err, "Failed to create job", "repo", repo.Name)

			continue
		}

		log.Info("Renovate job created", "job", job.Name, "repo", repo.Spec.Name)

		// Clean up old jobs
		if err := renovate.PruneJobHistory(
			ctx,
			r.Client,
			repo.Namespace,
			repoLabels,
			r.instance.Spec.SuccessLimit,
			r.instance.Spec.FailedLimit,
		); err != nil {
			log.Error(err, "Failed to clean up old jobs", "repo", repo.Name)
		}

		// Remove repo annotation if ad-hoc trigger occurred
		if hasRepoAnnotation {
			patch := client.MergeFrom(repo.DeepCopy())

			repo.Annotations = renovator.RemoveRenovatorOperation(repo.Annotations)
			if err := r.Patch(ctx, &repo, patch); err != nil {
				log.Error(err, "Failed to remove annotation", "repo", repo.Name)
			}
		}
	}

	return triggeredAny, nil
}

// updateJob configures the job spec for a GitRepo.
func (r *Reconciler) updateJob(job *batchv1.Job, repo *renovatev1beta1.GitRepo) {
	renovateConfigCM := metadata.GenericName(r.req, renovator.ConfigMapSuffix)

	// Set default job spec for the repository
	renovate.DefaultJobSpec(
		&job.Spec,
		r.renovate,
		renovateConfigCM,
		renovate.WithRepository(repo.Spec.Name),
	)

	// Configure job execution details
	job.Spec.Template.Spec.ServiceAccountName = metadata.GenericMetadata(r.req).Name
}

// evaluateSchedule checks if the scheduled run is due and returns the next run time.
func (r *Reconciler) evaluateSchedule() (bool, time.Time, error) {
	schedule, err := cron.ParseStandard(r.instance.Spec.Schedule)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("invalid schedule format: %w", err)
	}

	var lastRun time.Time
	if r.instance.Status.LastScheduleTime != nil {
		lastRun = r.instance.Status.LastScheduleTime.Time
	}

	nextRun := schedule.Next(lastRun)
	now := time.Now()

	// Check if the run is due
	if lastRun.IsZero() || now.After(nextRun) {
		return true, nextRun, nil
	}

	return false, nextRun, nil
}

// updateStatusAfterRun updates the runner's status after a run.
func (r *Reconciler) updateStatusAfterRun(ctx context.Context) (*ctrl.Result, error) {
	// Remove runner operation annotation if present
	if renovator.HasRenovatorOperationRenovate(r.instance.Annotations) {
		patch := client.MergeFrom(r.instance.DeepCopy())

		r.instance.Annotations = renovator.RemoveRenovatorOperation(r.instance.Annotations)
		if err := r.Patch(ctx, r.instance, patch); err != nil {
			return &ctrl.Result{}, fmt.Errorf("failed to remove annotation: %w", err)
		}
	}

	// Update last execution time
	r.instance.Status.LastScheduleTime = &metav1.Time{Time: time.Now()}
	if err := r.Status().Update(ctx, r.instance); err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	// Schedule next execution
	schedule, _ := cron.ParseStandard(r.instance.Spec.Schedule)
	nextRun := schedule.Next(time.Now())

	return &ctrl.Result{RequeueAfter: time.Until(nextRun)}, nil
}
