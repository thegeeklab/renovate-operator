package runner

import (
	"context"
	"fmt"
	"maps"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/metrics"
	"github.com/thegeeklab/renovate-operator/internal/parser"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
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

	runnerLabels, err := RunnerLabels(r.req)
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to build runner labels: %w", err)
	}

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

	maxParallel := r.instance.GetMaxParallel()

	var activeCount int

	if maxParallel > 0 {
		count, err := r.scheduler.CountActiveJobs(ctx, r.instance.Namespace, labels)
		if err != nil {
			return false, fmt.Errorf("failed to count active jobs: %w", err)
		}

		activeCount = count
	}

	for _, repo := range gitRepos.Items {
		repoLabels := make(map[string]string, len(labels)+1)
		maps.Copy(repoLabels, labels)

		repoLabel, err := k8s.SanitizeLabel(repo.Name)
		if err != nil {
			log.Error(err, "Failed to sanitize repo name for label", "repo", repo.Name)

			continue
		}

		repoLabels[renovatev1beta1.LabelGitRepo] = repoLabel

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

		if maxParallel > 0 && activeCount >= maxParallel {
			log.V(1).Info(
				"Max parallel jobs reached, skipping repo",
				"repo", repo.Name, "active", activeCount, "max", maxParallel,
			)

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

		activeCount++
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
// active. Status conditions are managed centrally by updateJobStatus.
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

	if err := r.updateJobStatus(ctx, repo, repoLabels); err != nil {
		return true, fmt.Errorf("failed to update job status condition: %w", err)
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
		renovate.WithImagePullSecrets(r.instance.Spec.ImagePullSecrets),
		renovate.WithPodSpec(r.instance.Spec.PodSpec),
		renovate.WithExtraEnv(r.instance.Spec.ExtraEnv),
		renovate.WithExtraVolumes(containers.WithRawVolumes(r.instance.Spec.ExtraVolumes)),
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
			hasActiveJob = true

			continue
		}

		if latestFinishedJob == nil || job.CreationTimestamp.After(latestFinishedJob.CreationTimestamp.Time) {
			latestFinishedJob = job
		}
	}

	patch := client.MergeFrom(repo.DeepCopy())

	if hasActiveJob {
		repo.SetCondition(
			renovatev1beta1.GitRepoConditionRenovateRunning,
			metav1.ConditionTrue,
			"JobActive", "Renovate job is running",
		)
	} else {
		repo.SetCondition(
			renovatev1beta1.GitRepoConditionRenovateRunning,
			metav1.ConditionFalse,
			"NoJobActive", "No renovate job is running",
		)
	}

	var (
		runStatus    string
		previousLast *metav1.Time
	)

	if latestFinishedJob != nil {
		previousLast = repo.GetLastRenovateTime()

		switch {
		case latestFinishedJob.Status.Succeeded > 0:
			repo.SetCondition(
				renovatev1beta1.GitRepoConditionRenovateCompleted,
				metav1.ConditionTrue,
				"JobSucceeded", "Renovate job completed successfully",
			)
			repo.RemoveCondition(renovatev1beta1.GitRepoConditionRenovateFailed)

			runStatus = metrics.StatusSucceeded
		case latestFinishedJob.Status.Failed > 0:
			repo.SetCondition(
				renovatev1beta1.GitRepoConditionRenovateFailed,
				metav1.ConditionTrue,
				"JobFailed", "Renovate job failed",
			)
			repo.RemoveCondition(renovatev1beta1.GitRepoConditionRenovateCompleted)

			runStatus = metrics.StatusFailed
		default:
			repo.RemoveCondition(renovatev1beta1.GitRepoConditionRenovateCompleted)
			repo.RemoveCondition(renovatev1beta1.GitRepoConditionRenovateFailed)

			logf.FromContext(ctx).Info(
				"finished job has no Succeeded/Failed counters, skipping metric emission",
				"job", latestFinishedJob.Name,
				"namespace", latestFinishedJob.Namespace,
				"succeeded", latestFinishedJob.Status.Succeeded,
				"failed", latestFinishedJob.Status.Failed,
			)
		}

		repo.SetLastRenovateTime(&latestFinishedJob.CreationTimestamp)
	}

	if err := r.Status().Patch(ctx, repo, patch); err != nil {
		return fmt.Errorf("failed to patch job status: %w", err)
	}

	if r.metrics != nil && latestFinishedJob != nil && runStatus != "" &&
		(previousLast == nil || latestFinishedJob.CreationTimestamp.After(previousLast.Time)) {
		renovatorLabel := repo.Labels[renovatev1beta1.LabelRenovator]
		gitrepoLabel, _ := k8s.SanitizeLabel(repo.Name)

		r.metrics.RecordGitRepoRun(
			repo.Namespace, renovatorLabel, r.instance.Name, gitrepoLabel, runStatus,
		)
		r.metrics.SetRunFailed(
			repo.Namespace, renovatorLabel, r.instance.Name, gitrepoLabel,
			runStatus == metrics.StatusFailed,
		)
		r.metrics.SetLastRunTimestamp(
			repo.Namespace, renovatorLabel, r.instance.Name, gitrepoLabel,
			float64(latestFinishedJob.CreationTimestamp.Unix()),
		)

		r.updateLogMetrics(ctx, latestFinishedJob, renovatorLabel, gitrepoLabel)
	}

	return nil
}

// updateLogMetrics parses the Renovate job logs and updates the
// dependency_issues and approvals_needed gauge metrics.
func (r *Reconciler) updateLogMetrics(
	ctx context.Context, job *batchv1.Job, renovatorLabel, gitrepoLabel string,
) {
	if r.logReader == nil {
		return
	}

	stream, err := r.logReader.ReadJobLogs(ctx, job.Namespace, job.Name, renovate.ContainerName, 0)
	if err != nil {
		logf.FromContext(ctx).V(1).Info(
			"Failed to read job logs for metrics", "job", job.Name, "error", err,
		)

		return
	}
	defer stream.Close()

	res, err := parser.ParseLogs(stream, -1)
	if err != nil {
		logf.FromContext(ctx).V(1).Info(
			"Failed to parse job logs for metrics", "job", job.Name, "error", err,
		)

		return
	}

	r.metrics.SetDependencyIssues(
		job.Namespace, renovatorLabel, r.instance.Name, gitrepoLabel, res.HasIssues,
	)
	r.metrics.SetApprovalsNeeded(
		job.Namespace, renovatorLabel, r.instance.Name, gitrepoLabel, res.PRActivity.NeedsApproval,
	)
}
