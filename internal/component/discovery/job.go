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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) reconcileJob(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Check if discovery is suspended
	if r.instance.Spec.Suspend != nil && *r.instance.Spec.Suspend {
		log.V(1).Info("Discovery is suspended, skipping")

		return &ctrl.Result{}, nil
	}

	// Check for active jobs (locking mechanism)
	discoveryLabels := DiscoveryMetadata(r.req).Labels

	activeJobs, err := renovate.GetActiveJobs(ctx, r.Client, r.instance.Namespace, discoveryLabels)
	if err != nil {
		return &ctrl.Result{}, err
	}

	if len(activeJobs) > 0 {
		log.V(1).Info("Active discovery job found, lock held. Requeuing.")

		return &ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Evaluate schedule to determine if job should run
	shouldRun, nextRun, err := r.evaluateSchedule()
	if err != nil {
		return &ctrl.Result{}, err
	}

	// Create and execute job if due
	if shouldRun {
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: DiscoveryName(r.req) + "-",
				Namespace:    r.instance.Namespace,
				Labels:       discoveryLabels,
			},
		}

		op, err := k8s.CreateOrUpdate(ctx, r.Client, job, r.instance, func() error {
			return r.updateJob(job)
		})
		if err != nil {
			return &ctrl.Result{}, err
		}

		if op != controllerutil.OperationResultNone {
			log.Info("Discovery job created", "job", job.Name)

			// Cleanup old jobs to maintain history limits
			if err := renovate.PruneJobHistory(
				ctx,
				r.Client,
				r.instance.Namespace,
				discoveryLabels,
				r.instance.Spec.SuccessLimit,
				r.instance.Spec.FailedLimit,
			); err != nil {
				log.Error(err, "failed to cleanup old discovery jobs")
			}

			// Update status and annotations after successful execution
			return r.updateStatusAfterRun(ctx)
		}
	}

	// Schedule next run if applicable
	now := time.Now()
	if nextRun.After(now) {
		log.V(1).Info("Next discovery scheduled", "time", nextRun, "wait", nextRun.Sub(now))

		return &ctrl.Result{RequeueAfter: nextRun.Sub(now)}, nil
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateJob(job *batchv1.Job) error {
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

	// Apply default job specification with init container
	renovate.DefaultJobSpec(
		&job.Spec,
		r.renovate,
		renovateConfigCM,
		renovate.WithInitContainer(initContainer),
		renovate.WithExtraVolumes(containers.WithEmptyDirVolume(renovate.VolumeRenovateTmp)),
	)

	// Configure main container for discovery operation
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

	return nil
}

func (r *Reconciler) evaluateSchedule() (bool, time.Time, error) {
	// Check for immediate execution annotation
	if renovator.HasRenovatorOperationDiscover(r.instance.Annotations) {
		return true, time.Now(), nil
	}

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

	// Determine if job should run now
	if lastRun.IsZero() || now.After(nextRun) {
		return true, nextRun, nil
	}

	return false, nextRun, nil
}

func (r *Reconciler) updateStatusAfterRun(ctx context.Context) (*ctrl.Result, error) {
	// Remove discovery operation annotation if present
	if renovator.HasRenovatorOperationDiscover(r.instance.Annotations) {
		r.instance.Annotations = renovator.RemoveRenovatorOperation(r.instance.Annotations)
		if err := r.Update(ctx, r.instance); err != nil {
			return &ctrl.Result{}, err
		}
	}

	// Update last execution time in status
	r.instance.Status.LastScheduleTime = &metav1.Time{Time: time.Now()}
	if err := r.Status().Update(ctx, r.instance); err != nil {
		return &ctrl.Result{}, err
	}

	// Schedule next execution based on configured schedule
	schedule, _ := cron.ParseStandard(r.instance.Spec.Schedule)
	nextRun := schedule.Next(time.Now())

	return &ctrl.Result{RequeueAfter: time.Until(nextRun)}, nil
}
