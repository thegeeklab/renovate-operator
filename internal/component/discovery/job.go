package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/netresearch/go-cron"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/discovery"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileJob checks if discovery should run, processes the job, and schedules the next run.
func (r *Reconciler) reconcileJob(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Evaluate if the scheduled run is due
	scheduleReady, nextRun, err := r.evaluateSchedule()
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to evaluate schedule: %w", err)
	}

	// Check for manual trigger and suspension state
	manualTrigger := renovator.HasRenovatorOperationDiscover(r.instance.Annotations)
	isSuspended := r.instance.Spec.Suspend != nil && *r.instance.Spec.Suspend

	// Determine if the job should run
	shouldRun := (scheduleReady && !isSuspended) || manualTrigger

	if isSuspended && scheduleReady && !manualTrigger {
		log.V(1).Info("Discovery is suspended: suppressing scheduled run")
	}

	// Create and execute job if triggered
	if shouldRun {
		// Check for active jobs (locking mechanism)
		discoveryLabels := DiscoveryMetadata(r.req).Labels

		activeJobs, err := renovate.GetActiveJobs(ctx, r.Client, r.instance.Namespace, discoveryLabels)
		if err != nil {
			return &ctrl.Result{}, fmt.Errorf("failed to check active jobs: %w", err)
		}

		if len(activeJobs) > 0 {
			log.V(1).Info("Active discovery job found: requeuing")

			return &ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}

		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: DiscoveryName(r.req) + "-",
				Namespace:    r.instance.Namespace,
				Labels:       discoveryLabels,
			},
		}
		r.updateJob(job)

		// Create the job
		if err := k8s.Create(ctx, r.Client, job, r.instance, nil); err != nil {
			return &ctrl.Result{}, fmt.Errorf("failed to create job: %w", err)
		}

		log.Info("Discovery job created", "job", job.Name)

		// Clean up old jobs
		if err := renovate.PruneJobHistory(
			ctx,
			r.Client,
			r.instance.Namespace,
			discoveryLabels,
			r.instance.Spec.SuccessLimit,
			r.instance.Spec.FailedLimit,
		); err != nil {
			log.Error(err, "Failed to clean up old discovery jobs")
		}

		// Update status and annotations
		return r.updateStatusAfterRun(ctx)
	}

	// Schedule next run
	now := time.Now()
	if nextRun.After(now) {
		log.V(1).Info("Next discovery scheduled", "time", nextRun, "wait", nextRun.Sub(now))

		return &ctrl.Result{RequeueAfter: nextRun.Sub(now)}, nil
	}

	return &ctrl.Result{}, nil
}

// updateJob configures the job spec for discovery.
func (r *Reconciler) updateJob(job *batchv1.Job) {
	renovateConfigCM := metadata.GenericName(r.req, renovator.ConfigMapSuffix)

	// Create init container for repository discovery
	initContainer := containers.ContainerTemplate(
		"renovate-init",
		r.renovate.Spec.Image,
		r.renovate.Spec.ImagePullPolicy,
		containers.WithContainerArgs([]string{
			"--write-discovered-repos",
			renovate.FileRenovateRepositories,
		}),
		containers.WithEnvVars(renovate.DefaultEnvVars(&r.renovate.Spec)),
		containers.WithEnvVars([]corev1.EnvVar{
			{Name: "RENOVATE_AUTODISCOVER", Value: "true"},
			{Name: "RENOVATE_AUTODISCOVER_FILTER", Value: strings.Join(r.instance.Spec.Filter, ",")},
		}),
		containers.WithVolumeMounts([]corev1.VolumeMount{
			{Name: renovate.VolumeRenovateTmp, MountPath: renovate.DirRenovateTmp},
			{Name: renovate.VolumeRenovateConfig, MountPath: renovate.DirRenovateConfig},
		}),
	)

	// Apply default job spec with init container
	renovate.DefaultJobSpec(
		&job.Spec,
		r.renovate,
		renovateConfigCM,
		renovate.WithInitContainer(initContainer),
		renovate.WithExtraVolumes(containers.WithEmptyDirVolume(renovate.VolumeRenovateTmp)),
	)

	// Configure main container for discovery
	job.Spec.Template.Spec.Containers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate-discovery",
			r.instance.Spec.Image,
			r.instance.Spec.ImagePullPolicy,
			containers.WithContainerCommand([]string{"/discovery"}),
			containers.WithEnvVars([]corev1.EnvVar{
				{Name: discovery.EnvDiscoveryInstanceName, Value: r.instance.Name},
				{Name: discovery.EnvDiscoveryInstanceNamespace, Value: r.instance.Namespace},
				{Name: discovery.EnvRenovateOutputFile, Value: renovate.FileRenovateRepositories},
			}),
			containers.WithVolumeMounts([]corev1.VolumeMount{
				{Name: renovate.VolumeRenovateTmp, MountPath: renovate.DirRenovateTmp},
			}),
		),
	}

	// Set service account for job execution
	job.Spec.Template.Spec.ServiceAccountName = metadata.GenericMetadata(r.req).Name
}

// evaluateSchedule checks if the job should run now and returns the next run time.
func (r *Reconciler) evaluateSchedule() (bool, time.Time, error) {
	// Parse schedule from specification
	schedule, err := cron.ParseStandard(r.instance.Spec.Schedule)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("invalid schedule format: %w", err)
	}

	// Determine last execution time
	var lastRun time.Time
	if r.instance.Status.LastScheduleTime != nil {
		lastRun = r.instance.Status.LastScheduleTime.Time
	}

	// Calculate next scheduled run time
	nextRun := schedule.Next(lastRun)
	now := time.Now()

	// Check if the run is due
	if lastRun.IsZero() || now.After(nextRun) {
		return true, nextRun, nil
	}

	return false, nextRun, nil
}

// updateStatusAfterRun updates the instance's status after a run.
func (r *Reconciler) updateStatusAfterRun(ctx context.Context) (*ctrl.Result, error) {
	// Remove discovery operation annotation if present
	if renovator.HasRenovatorOperationDiscover(r.instance.Annotations) {
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
