package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	"github.com/thegeeklab/renovate-operator/pkg/discovery"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileJob checks if discovery should run, processes the job, and schedules the next run.
func (r *Reconciler) reconcileJob(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	discoveryLabels := map[string]string{
		renovatev1beta1.RenovatorLabel: r.instance.Labels[renovatev1beta1.RenovatorLabel],
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
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: DiscoveryName(r.req) + "-",
				Namespace:    r.instance.Namespace,
				Labels:       discoveryLabels,
			},
		}
		r.updateJob(job, discoveryLabels)

		created, err := r.scheduler.EnsureJob(ctx, r.instance, job, discoveryLabels)
		if err != nil {
			return &ctrl.Result{}, fmt.Errorf("failed to ensure job: %w", err)
		}

		if !created {
			return &ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}

		log.Info("Discovery run active", "trigger", decision.Trigger)

		if err := r.scheduler.CompleteRun(ctx, r.instance, renovator.RemoveRenovatorOperation); err != nil {
			return &ctrl.Result{}, fmt.Errorf("failed to complete run: %w", err)
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

// updateJob configures the job spec for discovery.
func (r *Reconciler) updateJob(job *batchv1.Job, podLabels map[string]string) {
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
			{
				Name:  "RENOVATE_AUTODISCOVER",
				Value: "true",
			},
			{
				Name:  "RENOVATE_AUTODISCOVER_FILTER",
				Value: strings.Join(r.instance.Spec.Filter, ","),
			},
		}),
		containers.WithVolumeMounts([]corev1.VolumeMount{
			{
				Name:      renovate.VolumeRenovateTmp,
				MountPath: renovate.DirRenovateTmp,
			},
			{
				Name:      renovate.VolumeRenovateConfig,
				MountPath: renovate.DirRenovateConfig,
			},
		}),
	)

	// Apply default job spec with init container
	renovate.DefaultJobSpec(
		&job.Spec,
		r.renovate,
		renovateConfigCM,
		renovate.WithRenovateJobSpec(r.instance.Spec.JobSpec),
		renovate.WithPodLabels(podLabels),
		renovate.WithInitContainer(initContainer),
	)

	// Configure main container for discovery
	job.Spec.Template.Spec.Containers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate-discovery",
			r.instance.Spec.Image,
			r.instance.Spec.ImagePullPolicy,
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
					Value: renovate.FileRenovateRepositories,
				},
			}),
			containers.WithVolumeMounts([]corev1.VolumeMount{
				{
					Name:      renovate.VolumeRenovateTmp,
					MountPath: renovate.DirRenovateTmp,
				},
			}),
		),
	}

	// Set service account for job execution
	job.Spec.Template.Spec.ServiceAccountName = metadata.GenericMetadata(r.req).Name
}
