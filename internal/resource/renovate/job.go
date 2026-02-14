package renovate

import (
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// jobConfig holds the state required to build the JobSpec.
type jobConfig struct {
	// Context (Required)
	Renovate   *renovatev1beta1.RenovateConfig
	RenovateCM string

	// Modifiable Defaults
	Parallelism    *int32
	CompletionMode *batchv1.CompletionMode
	Completions    *int32

	// Accumulators
	InitContainers []corev1.Container
	VolMutators    []containers.VolumeMutator
	EnvVars        []corev1.EnvVar
}

// JobOption defines a function that modifies the job configuration.
type JobOption func(*jobConfig)

// DefaultJobSpec applies the Renovate job specification.
// It requires the base context arguments, followed by any number of JobOptions.
func DefaultJobSpec(
	spec *batchv1.JobSpec,
	renovate *renovatev1beta1.RenovateConfig,
	renovateCM string,
	opts ...JobOption,
) {
	// 1. Initialize Configuration with Safe Defaults
	cfg := &jobConfig{
		Renovate:   renovate,
		RenovateCM: renovateCM,

		// Base Volumes (Always present)
		VolMutators: []containers.VolumeMutator{
			containers.WithEmptyDirVolume(VolumeRenovateConfig),
			containers.WithConfigMapVolume(renovateCM, renovateCM),
		},

		// Base Env Vars (Always present)
		EnvVars: DefaultEnvVars(&renovate.Spec),
	}

	// 2. Apply all Functional Options
	for _, opt := range opts {
		opt(cfg)
	}

	// 3. Construct the Job Spec from the Config
	spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	spec.CompletionMode = cfg.CompletionMode
	spec.Completions = cfg.Completions
	spec.Parallelism = cfg.Parallelism
	spec.Template.Spec.InitContainers = cfg.InitContainers

	// Build Final Volume Slice
	spec.Template.Spec.Volumes = containers.VolumesTemplate(cfg.VolMutators...)

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

// WithIndexMode configures the job for a scheduled run.
func WithIndexMode(runner *renovatev1beta1.Runner, indexCM string, count int32) JobOption {
	return func(c *jobConfig) {
		c.Parallelism = ptr.To(runner.Spec.Instances)
		c.CompletionMode = ptr.To(batchv1.IndexedCompletion)
		c.Completions = ptr.To(count)

		c.VolMutators = append(c.VolMutators, containers.WithConfigMapVolume(indexCM, indexCM))

		c.InitContainers = []corev1.Container{
			containers.ContainerTemplate(
				"renovate-dispatcher",
				runner.Spec.Image,
				runner.Spec.ImagePullPolicy,
				containers.WithEnvVars([]corev1.EnvVar{
					{Name: EnvRenovateConfigRaw, Value: FileRenovateTmp},
					{Name: EnvRenovateConfig, Value: FileRenovateConfig},
					{Name: EnvRenovateIndex, Value: FileRenovateIndex},
				}),
				containers.WithContainerCommand([]string{"/dispatcher"}),
				containers.WithVolumeMounts([]corev1.VolumeMount{
					{Name: VolumeRenovateConfig, MountPath: DirRenovateConfig},
					{Name: c.RenovateCM, ReadOnly: true, MountPath: FileRenovateTmp, SubPath: FilenameRenovateConfig},
					{Name: indexCM, ReadOnly: true, MountPath: FileRenovateIndex, SubPath: FilenameIndex},
				}),
			),
		}
	}
}

// WithSingleRepoMode configures the job for a single repository run.
func WithSingleRepoMode(targetRepo string) JobOption {
	return func(c *jobConfig) {
		// Single run requires no indexing and no parallelism
		c.CompletionMode = ptr.To(batchv1.NonIndexedCompletion)
		c.Completions = ptr.To(int32(1))
		c.Parallelism = ptr.To(int32(1))

		// Inject the override environment variable
		c.EnvVars = append(c.EnvVars, corev1.EnvVar{
			Name:  "RENOVATE_REPOSITORIES",
			Value: targetRepo,
		})
	}
}
