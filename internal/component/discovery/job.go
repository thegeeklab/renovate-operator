package discovery

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/metrics"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	"github.com/thegeeklab/renovate-operator/internal/resource/job"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	"github.com/thegeeklab/renovate-operator/pkg/discovery"
	"github.com/thegeeklab/renovate-operator/pkg/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var errJobNotCreated = errors.New("job was not created")

// reconcileJob checks if discovery should run, processes the job, and schedules the next run.
func (r *Reconciler) reconcileJob(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	discoveryLabels, err := DiscoveryLabels(r.req)
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to build discovery labels: %w", err)
	}

	if val, ok := r.instance.Labels[renovatev1beta1.LabelRenovator]; ok {
		discoveryLabels[renovatev1beta1.LabelRenovator] = val
	}

	if err := r.updateJobStatus(ctx, discoveryLabels); err != nil {
		log.Error(err, "Failed to update job status")
	}

	if err := r.scheduler.PruneJobs(
		ctx, r.instance.Namespace, discoveryLabels, r.instance.GetSuccessLimit(), r.instance.GetFailedLimit(),
	); err != nil {
		log.Error(err, "Failed to prune discovery jobs")
	}

	decision, err := r.scheduler.Evaluate(r.instance, renovator.HasRenovatorOperationDiscover)
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to evaluate schedule: %w", err)
	}

	if decision.Trigger == scheduler.TriggerSuspended {
		log.V(1).Info("Discovery is suspended: suppressing scheduled run")
	}

	if decision.ShouldRun {
		if err := r.runDiscoveryJob(ctx, discoveryLabels, decision.Trigger); err != nil {
			if errors.Is(err, errJobNotCreated) {
				return &ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
			}

			return &ctrl.Result{}, fmt.Errorf("failed to run discovery job: %w", err)
		}
	}

	nextDecision, err := r.scheduler.Evaluate(r.instance, renovator.HasRenovatorOperationDiscover)
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to re-evaluate schedule: %w", err)
	}

	now := time.Now()
	if nextDecision.NextRun.After(now) {
		waitDuration := nextDecision.NextRun.Sub(now)
		log.V(1).Info("Next discovery scheduled", "time", nextDecision.NextRun, "wait", waitDuration)

		return &ctrl.Result{RequeueAfter: waitDuration}, nil
	}

	return &ctrl.Result{}, nil
}

// runDiscoveryJob creates and dispatches a discovery job.
func (r *Reconciler) runDiscoveryJob(ctx context.Context, discoveryLabels map[string]string, trigger string) error {
	log := logf.FromContext(ctx)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: DiscoveryName(r.req) + "-",
			Namespace:    r.instance.Namespace,
			Labels:       discoveryLabels,
		},
	}

	if err := r.updateJob(job, discoveryLabels); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	created, err := r.scheduler.EnsureJob(ctx, r.instance, job, discoveryLabels)
	if err != nil {
		return fmt.Errorf("failed to ensure job: %w", err)
	}

	if !created {
		return errJobNotCreated
	}

	if r.metrics != nil {
		renovatorLabel := r.instance.Labels[renovatev1beta1.LabelRenovator]
		r.metrics.RecordDiscoveryJob(r.instance.Namespace, renovatorLabel, r.instance.Name, metrics.StatusDispatched)
	}

	log.Info("Discovery run active", "trigger", trigger)

	if err := r.scheduler.CompleteRun(ctx, r.instance, renovator.RemoveRenovatorOperation); err != nil {
		return fmt.Errorf("failed to complete run: %w", err)
	}

	return nil
}

// updateJob configures the job spec for discovery.
func (r *Reconciler) updateJob(job *batchv1.Job, podLabels map[string]string) error {
	renovateConfigCM := metadata.GenericName(r.req, renovator.ConfigMapSuffix)
	scratchPath := renovate.GetScratchVolumePath(r.instance.Spec.ScratchVolume)
	reposFile := filepath.Join(scratchPath, renovate.FilenameRepositories)

	if len(r.instance.Spec.PodLabelTemplates) > 0 {
		vars := map[string]string{
			"namespace": r.instance.Namespace,
			"renovator": r.instance.Labels[renovatev1beta1.LabelRenovator],
			"discovery": r.instance.Name,
		}

		if err := util.MergeRenderedPodLabels(podLabels, r.instance.Spec.PodLabelTemplates, vars); err != nil {
			return fmt.Errorf("failed to render pod label templates: %w", err)
		}
	}

	// Build scratch mounts for discovery containers (volume is created by DefaultJobSpec)
	_, scratchMounts := renovate.BuildScratchVolumeAndMounts(r.instance.Spec.ScratchVolume)

	initContainer := containers.ContainerTemplate(
		"renovate-init",
		r.renovate.Spec.Image,
		r.renovate.Spec.ImagePullPolicy,
		containers.WithContainerArgs([]string{
			"--write-discovered-repos",
			reposFile,
		}),
		containers.WithEnvVars(renovate.DefaultEnvVars(&r.renovate.Spec)),
		containers.WithEnvVars([]corev1.EnvVar{
			{
				Name:  "RENOVATE_AUTODISCOVER",
				Value: "true",
			},
			{
				Name:  "RENOVATE_AUTODISCOVER_FILTER",
				Value: strings.Join(r.instance.Spec.Filter, ","),
			},
		}),
		containers.WithVolumeMounts(append(scratchMounts, corev1.VolumeMount{
			Name:      renovate.VolumeRenovateConfig,
			MountPath: renovate.DirRenovateConfig,
		})),
	)

	renovate.DefaultJobSpec(
		&job.Spec,
		r.renovate,
		renovateConfigCM,
		renovate.WithRenovateJobSpec(r.instance.Spec.JobSpec),
		renovate.WithPodLabels(podLabels),
		renovate.WithInitContainer(initContainer),
		renovate.WithImagePullSecrets(r.instance.Spec.ImagePullSecrets),
		renovate.WithPodSpec(r.instance.Spec.PodSpec),
		renovate.WithExtraEnv(r.instance.Spec.ExtraEnv),
		renovate.WithExtraVolumes(containers.WithRawVolumes(r.instance.Spec.ExtraVolumes)),
	)

	discoveryMutators := []containers.ContainerMutator{
		containers.WithContainerCommand([]string{"/discovery"}),
		containers.WithEnvVars([]corev1.EnvVar{
			{
				Name:  discovery.EnvDiscoveryInstanceName,
				Value: r.instance.Name,
			},
			{
				Name:  discovery.EnvDiscoveryInstanceNamespace,
				Value: r.instance.Namespace,
			},
			{
				Name:  discovery.EnvRenovateOutputFile,
				Value: reposFile,
			},
		}),
		containers.WithEnvVars(r.instance.Spec.ExtraEnv),
		containers.WithVolumeMounts(scratchMounts),
	}

	if r.instance.Spec.Resources.Limits != nil || r.instance.Spec.Resources.Requests != nil {
		discoveryMutators = append(discoveryMutators, containers.WithResourceRequirements(r.instance.Spec.Resources))
	}

	if r.instance.Spec.SecurityContext != nil {
		discoveryMutators = append(discoveryMutators, containers.WithSecurityContext(r.instance.Spec.SecurityContext))
	}

	job.Spec.Template.Spec.Containers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate-discovery",
			r.instance.Spec.Image,
			r.instance.Spec.ImagePullPolicy,
			discoveryMutators...,
		),
	}

	job.Spec.Template.Spec.ServiceAccountName = metadata.GenericMetadata(r.req).Name

	return nil
}

// updateJobStatus lists jobs for this discovery and updates the Discovery
// status conditions (JobRunning, JobCompleted, JobFailed) based on the
// current job state.
func (r *Reconciler) updateJobStatus(ctx context.Context, labels map[string]string) error {
	var jobList batchv1.JobList

	if err := r.List(ctx, &jobList, client.InNamespace(r.instance.Namespace), client.MatchingLabels(labels)); err != nil {
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

	patch := client.MergeFrom(r.instance.DeepCopy())

	if hasActiveJob {
		r.instance.SetCondition(
			renovatev1beta1.DiscoveryConditionDiscoveryRunning,
			metav1.ConditionTrue,
			"JobActive", "Discovery job is running",
		)
	} else {
		r.instance.SetCondition(
			renovatev1beta1.DiscoveryConditionDiscoveryRunning,
			metav1.ConditionFalse,
			"NoJobActive", "No discovery job is running",
		)
	}

	var (
		runStatus    string
		previousLast *metav1.Time
	)

	if latestFinishedJob != nil {
		previousLast = r.instance.GetLastDiscoveryTime()

		switch {
		case latestFinishedJob.Status.Succeeded > 0:
			r.instance.SetCondition(
				renovatev1beta1.DiscoveryConditionDiscoveryCompleted,
				metav1.ConditionTrue,
				"JobSucceeded", "Discovery job completed successfully",
			)
			r.instance.RemoveCondition(renovatev1beta1.DiscoveryConditionDiscoveryFailed)

			runStatus = metrics.StatusSucceeded
		case latestFinishedJob.Status.Failed > 0:
			r.instance.SetCondition(
				renovatev1beta1.DiscoveryConditionDiscoveryFailed,
				metav1.ConditionTrue,
				"JobFailed", "Discovery job failed",
			)
			r.instance.RemoveCondition(renovatev1beta1.DiscoveryConditionDiscoveryCompleted)

			runStatus = metrics.StatusFailed
		default:
			r.instance.RemoveCondition(renovatev1beta1.DiscoveryConditionDiscoveryCompleted)
			r.instance.RemoveCondition(renovatev1beta1.DiscoveryConditionDiscoveryFailed)
		}

		r.instance.SetLastDiscoveryTime(&latestFinishedJob.CreationTimestamp)
	}

	if err := r.Status().Patch(ctx, r.instance, patch); err != nil {
		return fmt.Errorf("failed to patch job status: %w", err)
	}

	if r.metrics != nil && latestFinishedJob != nil && runStatus != "" &&
		(previousLast == nil || latestFinishedJob.CreationTimestamp.After(previousLast.Time)) {
		renovatorLabel := r.instance.Labels[renovatev1beta1.LabelRenovator]
		r.metrics.RecordDiscoveryJob(r.instance.Namespace, renovatorLabel, r.instance.Name, runStatus)

		if runStatus == metrics.StatusFailed {
			r.metrics.RecordDiscoveryJobFailure(
				r.instance.Namespace, renovatorLabel, r.instance.Name, job.FailureReason(latestFinishedJob),
			)
		}
	}

	return nil
}
