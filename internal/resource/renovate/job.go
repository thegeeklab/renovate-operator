package renovate

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// jobConfig holds the state required to build the JobSpec.
type jobConfig struct {
	Renovate                *renovatev1beta1.RenovateConfig
	RenovateCM              string
	BackoffLimit            *int32
	TTLSecondsAfterFinished *int32

	InitContainers []corev1.Container
	VolumeMutators []containers.VolumeMutator
	EnvVars        []corev1.EnvVar
	PodLabels      map[string]string
}

// JobOption defines a function that modifies the job configuration.
type JobOption func(*jobConfig)

// DefaultJobSpec applies the Renovate job specification.
func DefaultJobSpec(
	spec *batchv1.JobSpec, renovate *renovatev1beta1.RenovateConfig, renovateCM string, opts ...JobOption,
) {
	// 1. Initialize Configuration with Safe Defaults
	cfg := &jobConfig{
		Renovate:   renovate,
		RenovateCM: renovateCM,
		VolumeMutators: []containers.VolumeMutator{
			containers.WithEmptyDirVolume(VolumeRenovateTmp),
			containers.WithConfigMapVolume(VolumeRenovateConfig, renovateCM),
		},
		EnvVars: DefaultEnvVars(&renovate.Spec),
	}

	// 2. Apply all Functional Options
	for _, opt := range opts {
		opt(cfg)
	}

	// 3. Construct the Job Spec from the Config
	spec.CompletionMode = ptr.To(batchv1.NonIndexedCompletion)
	spec.Parallelism = ptr.To(int32(1))
	spec.BackoffLimit = cfg.BackoffLimit
	spec.TTLSecondsAfterFinished = cfg.TTLSecondsAfterFinished

	spec.Template.Labels = cfg.PodLabels
	spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	spec.Template.Spec.InitContainers = cfg.InitContainers
	spec.Template.Spec.Volumes = containers.VolumesTemplate(cfg.VolumeMutators...)

	// Build Main Container
	spec.Template.Spec.Containers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate",
			renovate.Spec.Image,
			renovate.Spec.ImagePullPolicy,
			containers.WithEnvVars(cfg.EnvVars),
			containers.WithVolumeMounts([]corev1.VolumeMount{
				{
					Name:      VolumeRenovateConfig,
					MountPath: DirRenovateConfig,
				},
			}),
		),
	}
}

// WithPodLabels injects custom labels into the Pod template.
func WithPodLabels(labels map[string]string) JobOption {
	return func(c *jobConfig) {
		c.PodLabels = labels
	}
}

// WithRepository configures the job to run against a specific repository.
func WithRepository(targetRepo string) JobOption {
	return func(c *jobConfig) {
		c.EnvVars = append(c.EnvVars, corev1.EnvVar{
			Name:  "RENOVATE_REPOSITORIES",
			Value: targetRepo,
		})
	}
}

// WithInitContainer allows injecting an InitContainer.
func WithInitContainer(container corev1.Container) JobOption {
	return func(c *jobConfig) {
		c.InitContainers = append(c.InitContainers, container)
	}
}

// WithExtraVolumes allows injecting extra volumes.
func WithExtraVolumes(mutators ...containers.VolumeMutator) JobOption {
	return func(c *jobConfig) {
		c.VolumeMutators = append(c.VolumeMutators, mutators...)
	}
}

// WithExtraEnv allows injecting ad-hoc environment variables.
func WithExtraEnv(env []corev1.EnvVar) JobOption {
	return func(c *jobConfig) {
		c.EnvVars = append(c.EnvVars, env...)
	}
}

// WithRenovateJobSpec applies the operator's Job configuration to the Kubernetes Job.
func WithRenovateJobSpec(js renovatev1beta1.JobSpec) JobOption {
	return func(c *jobConfig) {
		if js.BackoffLimit != nil {
			c.BackoffLimit = js.BackoffLimit
		}

		if js.TTLSecondsAfterFinished != nil {
			c.TTLSecondsAfterFinished = js.TTLSecondsAfterFinished
		}
	}
}

// GetActiveJobs returns a list of running jobs matching the given labels.
func GetActiveJobs(
	ctx context.Context, c client.Client, namespace string, labels map[string]string,
) ([]batchv1.Job, error) {
	jobList := &batchv1.JobList{}

	err := c.List(ctx, jobList,
		client.InNamespace(namespace),
		client.MatchingLabels(labels),
	)
	if err != nil {
		return nil, err
	}

	var active []batchv1.Job

	for _, job := range jobList.Items {
		if job.Status.Active > 0 {
			active = append(active, job)
		}
	}

	return active, nil
}
