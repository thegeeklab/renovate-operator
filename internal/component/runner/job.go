package runner

import (
	"context"
	"fmt"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/component/scheduler"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
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

	decision, err := r.scheduler.Evaluate(r.instance, renovator.HasRenovatorOperationRenovate)
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to evaluate schedule: %w", err)
	}

	if decision.Trigger == scheduler.TriggerSuspended {
		log.V(1).Info("Runner is suspended: suppressing scheduled run")
	}

	// Process all GitRepo resources
	triggeredAny, err := r.processGitRepos(ctx, decision.ShouldRun)
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to process GitRepos: %w", err)
	}

	// Update status and annotations if a global run was triggered
	if decision.ShouldRun {
		log.Info("Runner run active", "trigger", decision.Trigger)

		if err := r.scheduler.CompleteRun(ctx, r.instance, renovator.RemoveRenovatorOperation); err != nil {
			return &ctrl.Result{}, fmt.Errorf("failed to complete run: %w", err)
		}
	} else if triggeredAny {
		// If only individual repos were triggered, return without rescheduling.
		// The GitRepo annotation patch inside processGitRepos will trigger a new reconciliation.
		return &ctrl.Result{}, nil
	}

	// Schedule the next run
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

		repoLabels := map[string]string{
			renovatev1beta1.RenovatorLabel:   r.instance.Labels[renovatev1beta1.RenovatorLabel],
			"renovate.thegeeklab.de/gitrepo": repo.Name,
		}

		// Clean up old jobs for this repo
		if err := r.scheduler.PruneJobs(
			ctx, repo.Namespace, repoLabels, r.instance.GetSuccessLimit(), r.instance.GetFailedLimit(),
		); err != nil {
			log.Error(err, "Failed to clean up old jobs", "repo", repo.Name)
		}

		// Prepare job
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: repo.Name + "-",
				Namespace:    repo.Namespace,
				Labels:       repoLabels,
			},
		}
		r.updateJob(job, &repo)

		// Ensure the job runs using the scheduler
		created, err := r.scheduler.EnsureJob(ctx, r.instance, job, repoLabels)
		if err != nil {
			log.Error(err, "Failed to ensure job", "repo", repo.Name)

			continue
		}

		if !created {
			log.V(1).Info("Active renovate job found: skipping", "repo", repo.Name)

			continue
		}

		log.Info("Renovate job created", "job", job.Name, "repo", repo.Spec.Name)

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
		&job.Spec, r.renovate, renovateConfigCM, renovate.WithRepository(repo.Spec.Name),
	)

	// Configure job execution details
	job.Spec.Template.Spec.ServiceAccountName = metadata.GenericMetadata(r.req).Name
}
