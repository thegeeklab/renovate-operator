package runner

import (
	"context"
	"fmt"
	"maps"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileJob determines if a global run is due, processes GitRepo resources,
// and schedules the next run if necessary.
func (r *Reconciler) reconcileJob(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	runnerLabels := RunnerLabels(r.req)

	if val, ok := r.instance.Labels[renovatev1beta1.LabelRenovator]; ok {
		runnerLabels[renovatev1beta1.LabelRenovator] = val
	}

	decision, err := r.scheduler.Evaluate(r.instance, renovator.HasRenovatorOperationRenovate)
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to evaluate schedule: %w", err)
	}

	// Process all GitRepo resources
	triggeredAny, err := r.processGitRepos(ctx, decision.ShouldRun, runnerLabels)
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to process GitRepos: %w", err)
	}

	if decision.Trigger == scheduler.TriggerSuspended && !triggeredAny {
		log.V(1).Info("Runner is suspended: suppressing scheduled run")
	}

	if decision.ShouldRun {
		log.Info("Runner run active", "trigger", decision.Trigger)

		if err := r.scheduler.CompleteRun(ctx, r.instance, renovator.RemoveRenovatorOperation); err != nil {
			return &ctrl.Result{}, fmt.Errorf("failed to complete run: %w", err)
		}
	}

	nextDecision, err := r.scheduler.Evaluate(r.instance, renovator.HasRenovatorOperationRenovate)
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to re-evaluate schedule: %w", err)
	}

	now := time.Now()
	if nextDecision.NextRun.After(now) {
		waitDuration := nextDecision.NextRun.Sub(now)
		log.V(1).Info("Next runner execution scheduled", "time", nextDecision.NextRun, "wait", waitDuration)

		return &ctrl.Result{RequeueAfter: waitDuration}, nil
	}

	return &ctrl.Result{}, nil
}

// processGitRepos processes each GitRepo and creates jobs if needed.
func (r *Reconciler) processGitRepos(
	ctx context.Context, isGlobalTrigger bool, labels map[string]string,
) (bool, error) {
	log := logf.FromContext(ctx)
	triggeredAny := false

	gitRepos := &renovatev1beta1.GitRepoList{}

	listOpts := []client.ListOption{
		client.InNamespace(r.instance.Namespace),
	}
	if val, ok := labels[renovatev1beta1.LabelRenovator]; ok {
		listOpts = append(listOpts, client.MatchingLabels{
			renovatev1beta1.LabelRenovator: val,
		})
	}

	if err := r.List(ctx, gitRepos, listOpts...); err != nil {
		return false, fmt.Errorf("failed to list GitRepos: %w", err)
	}

	for _, repo := range gitRepos.Items {
		repoLabels := make(map[string]string, len(labels)+1)
		maps.Copy(repoLabels, labels)
		repoLabels[renovatev1beta1.LabelGitRepo] = repo.Name

		if err := r.updateJobStatus(ctx, &repo, repoLabels); err != nil {
			log.Error(err, "Failed to update job status", "repo", repo.Name)
		}

		if err := r.scheduler.PruneJobs(
			ctx, repo.Namespace, repoLabels, r.instance.GetSuccessLimit(), r.instance.GetFailedLimit(),
		); err != nil {
			log.Error(err, "Failed to clean up old jobs", "repo", repo.Name)
		}

		hasRepoAnnotation := renovator.HasRenovatorOperationRenovate(repo.Annotations)
		if !isGlobalTrigger && !hasRepoAnnotation {
			continue
		}

		created, err := r.ensureRepoJob(ctx, &repo, repoLabels)
		if err != nil {
			log.Error(err, "Failed to ensure job", "repo", repo.Name)

			continue
		}

		if !created {
			log.V(1).Info("Active renovate job found: skipping", "repo", repo.Name)

			continue
		}

		triggeredAny = true

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

// ensureRepoJob creates a renovate job for the given repository when none is
// active and updates the GitRepo status to reflect the new run.
func (r *Reconciler) ensureRepoJob(
	ctx context.Context, repo *renovatev1beta1.GitRepo, repoLabels map[string]string,
) (bool, error) {
	log := logf.FromContext(ctx)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: repo.Name + "-",
			Namespace:    repo.Namespace,
			Labels:       repoLabels,
		},
	}
	r.updateJob(job, repo, repoLabels)

	created, err := r.scheduler.EnsureJob(ctx, r.instance, job, repoLabels)
	if err != nil {
		return false, err
	}

	if !created {
		return false, nil
	}

	log.Info("Renovate job created", "job", job.Name, "repo", repo.Spec.Name)

	patch := client.MergeFrom(repo.DeepCopy())
	now := metav1.Now()
	repo.SetCondition(
		renovatev1beta1.GitRepoConditionRenovateRunning,
		metav1.ConditionTrue,
		"JobCreated", "Renovate job is running", now,
	)
	repo.RemoveCondition(renovatev1beta1.GitRepoConditionRenovateCompleted)
	repo.RemoveCondition(renovatev1beta1.GitRepoConditionRenovateFailed)

	if err := r.Status().Patch(ctx, repo, patch); err != nil {
		log.Error(err, "Failed to update job status condition", "repo", repo.Name)
	}

	return true, nil
}

// updateJob configures the job spec for a GitRepo.
func (r *Reconciler) updateJob(job *batchv1.Job, repo *renovatev1beta1.GitRepo, podLabels map[string]string) {
	renovateConfigCM := metadata.GenericName(r.req, renovator.ConfigMapSuffix)

	// Set default job spec for the repository
	renovate.DefaultJobSpec(
		&job.Spec,
		r.renovate,
		renovateConfigCM,
		renovate.WithRenovateJobSpec(r.instance.Spec.JobSpec),
		renovate.WithPodLabels(podLabels),
		renovate.WithRepository(repo.Spec.Name),
	)

	// Configure job execution details
	job.Spec.Template.Spec.ServiceAccountName = metadata.GenericMetadata(r.req).Name
}

// updateJobStatus checks for jobs and updates the GitRepo's status conditions
// and LastRenovateTime based on the most recent job state.
func (r *Reconciler) updateJobStatus(
	ctx context.Context, repo *renovatev1beta1.GitRepo, labels map[string]string,
) error {
	var jobList batchv1.JobList

	if err := r.List(ctx, &jobList, client.InNamespace(repo.Namespace), client.MatchingLabels(labels)); err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	var (
		latestFinishedJob *batchv1.Job
		hasActiveJob      bool
	)

	for i := range jobList.Items {
		job := &jobList.Items[i]

		if !scheduler.IsJobFinished(job) {
			if job.Status.Active > 0 {
				hasActiveJob = true
			}

			continue
		}

		if latestFinishedJob == nil || job.CreationTimestamp.After(latestFinishedJob.CreationTimestamp.Time) {
			latestFinishedJob = job
		}
	}

	patch := client.MergeFrom(repo.DeepCopy())
	now := metav1.Now()

	if hasActiveJob {
		repo.SetCondition(
			renovatev1beta1.GitRepoConditionRenovateRunning,
			metav1.ConditionTrue,
			"JobActive", "Renovate job is running", now,
		)
	} else {
		repo.SetCondition(
			renovatev1beta1.GitRepoConditionRenovateRunning,
			metav1.ConditionFalse,
			"NoJobActive", "No renovate job is running", now,
		)
	}

	if latestFinishedJob != nil {
		if latestFinishedJob.Status.Succeeded > 0 {
			repo.SetCondition(
				renovatev1beta1.GitRepoConditionRenovateCompleted,
				metav1.ConditionTrue,
				"JobSucceeded", "Renovate job completed successfully", now,
			)
			repo.RemoveCondition(renovatev1beta1.GitRepoConditionRenovateFailed)
			repo.SetLastRenovateTime(&latestFinishedJob.CreationTimestamp)
		} else if latestFinishedJob.Status.Failed > 0 {
			repo.SetCondition(
				renovatev1beta1.GitRepoConditionRenovateFailed,
				metav1.ConditionTrue,
				"JobFailed", "Renovate job failed", now,
			)
			repo.RemoveCondition(renovatev1beta1.GitRepoConditionRenovateCompleted)
			repo.SetLastRenovateTime(&latestFinishedJob.CreationTimestamp)
		}
	}

	if err := r.Status().Patch(ctx, repo, patch); err != nil {
		return fmt.Errorf("failed to patch job status: %w", err)
	}

	return nil
}
