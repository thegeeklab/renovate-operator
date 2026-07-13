package renovate

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// jobConfig holds the state required to build the JobSpec.
type jobConfig struct {
	Renovate                *renovatev1beta1.RenovateConfig
	RenovateCM              string
	BackoffLimit            *int32
	TTLSecondsAfterFinished *int32

	InitContainers            []corev1.Container
	VolumeMutators            []containers.VolumeMutator
	EnvVars                   []corev1.EnvVar
	PodLabels                 map[string]string
	PodAnnotations            map[string]string
	ImagePullSecrets          []corev1.LocalObjectReference
	NodeSelector              map[string]string
	Affinity                  *corev1.Affinity
	Tolerations               []corev1.Toleration
	TopologySpreadConstraints []corev1.TopologySpreadConstraint
	Resources                 corev1.ResourceRequirements
	SecurityContext           *corev1.SecurityContext
	RuntimeClassName          *string
	ScratchVolume             *renovatev1beta1.ScratchVolumeSpec
}

// JobOption defines a function that modifies the job configuration.
type JobOption func(*jobConfig)

// DefaultJobSpec applies the Renovate job specification.
func DefaultJobSpec(
	spec *batchv1.JobSpec, renovate *renovatev1beta1.RenovateConfig, renovateCM string, opts ...JobOption,
) {
	// Initialize Configuration with Safe Defaults
	cfg := &jobConfig{
		Renovate:         renovate,
		RenovateCM:       renovateCM,
		VolumeMutators:   []containers.VolumeMutator{containers.WithConfigMapVolume(VolumeRenovateConfig, renovateCM)},
		EnvVars:          DefaultEnvVars(&renovate.Spec),
		ImagePullSecrets: append([]corev1.LocalObjectReference(nil), renovate.Spec.ImagePullSecrets...),
	}

	// Apply all Functional Options
	for _, opt := range opts {
		opt(cfg)
	}

	// Build scratch volume and mounts
	scratchVolumes, scratchMounts := BuildScratchVolumeAndMounts(cfg.ScratchVolume)
	cfg.VolumeMutators = append(cfg.VolumeMutators, scratchVolumes...)

	// Add RENOVATE_BASE_DIR env var if scratch volume is enabled
	if cfg.ScratchVolume == nil || cfg.ScratchVolume.Enabled {
		cfg.EnvVars = append(cfg.EnvVars, corev1.EnvVar{
			Name:  "RENOVATE_BASE_DIR",
			Value: GetScratchVolumePath(cfg.ScratchVolume),
		})
	}

	// Construct the Job Spec from the Config
	spec.CompletionMode = new(batchv1.NonIndexedCompletion)
	spec.Parallelism = new(int32(1))
	spec.BackoffLimit = cfg.BackoffLimit
	spec.TTLSecondsAfterFinished = cfg.TTLSecondsAfterFinished

	spec.Template.Labels = cfg.PodLabels
	spec.Template.Annotations = cfg.PodAnnotations
	spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	spec.Template.Spec.InitContainers = cfg.InitContainers
	spec.Template.Spec.Volumes = containers.VolumesTemplate(cfg.VolumeMutators...)
	spec.Template.Spec.ImagePullSecrets = cfg.ImagePullSecrets
	spec.Template.Spec.NodeSelector = cfg.NodeSelector
	spec.Template.Spec.Affinity = cfg.Affinity
	spec.Template.Spec.Tolerations = cfg.Tolerations
	spec.Template.Spec.TopologySpreadConstraints = cfg.TopologySpreadConstraints
	spec.Template.Spec.RuntimeClassName = cfg.RuntimeClassName

	// Build Main Container
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      VolumeRenovateConfig,
			MountPath: DirRenovateConfig,
		},
	}
	volumeMounts = append(volumeMounts, scratchMounts...)

	containerMutators := []containers.ContainerMutator{
		containers.WithEnvVars(cfg.EnvVars),
		containers.WithVolumeMounts(volumeMounts),
	}

	if cfg.Resources.Limits != nil || cfg.Resources.Requests != nil {
		containerMutators = append(containerMutators, containers.WithResourceRequirements(cfg.Resources))
	}

	if cfg.SecurityContext != nil {
		containerMutators = append(containerMutators, containers.WithSecurityContext(cfg.SecurityContext))
	}

	spec.Template.Spec.Containers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate",
			renovate.Spec.Image,
			renovate.Spec.ImagePullPolicy,
			containerMutators...,
		),
	}
}

// WithPodLabels injects custom labels into the Pod template.
func WithPodLabels(labels map[string]string) JobOption {
	return func(c *jobConfig) {
		c.PodLabels = labels
	}
}

// WithPodSpec applies pod-level scheduling configuration.
func WithPodSpec(podSpec renovatev1beta1.PodSpec) JobOption {
	return func(c *jobConfig) {
		c.NodeSelector = podSpec.NodeSelector
		c.Affinity = podSpec.Affinity
		c.Tolerations = podSpec.Tolerations
		c.TopologySpreadConstraints = podSpec.TopologySpreadConstraints
		c.Resources = podSpec.Resources
		c.SecurityContext = podSpec.SecurityContext
		c.RuntimeClassName = podSpec.RuntimeClassName
		c.PodAnnotations = podSpec.PodAnnotations
		c.ScratchVolume = podSpec.ScratchVolume
	}
}

// WithResources sets the resource requirements for the renovate container.
func WithResources(resources corev1.ResourceRequirements) JobOption {
	return func(c *jobConfig) {
		c.Resources = resources
	}
}

// WithSecurityContext sets the security context for the renovate container.
func WithSecurityContext(sc *corev1.SecurityContext) JobOption {
	return func(c *jobConfig) {
		c.SecurityContext = sc
	}
}

// WithImagePullSecrets injects image pull secrets into the Pod template.
func WithImagePullSecrets(secrets []corev1.LocalObjectReference) JobOption {
	return func(c *jobConfig) {
		c.ImagePullSecrets = append(c.ImagePullSecrets, secrets...)
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

	err := c.List(
		ctx, jobList,
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

// BuildScratchVolumeAndMounts creates the scratch volume and mount based on the spec.
// Returns empty slices if scratch volume is disabled.
func BuildScratchVolumeAndMounts(
	scratch *renovatev1beta1.ScratchVolumeSpec,
) ([]containers.VolumeMutator, []corev1.VolumeMount) {
	if scratch != nil && !scratch.Enabled {
		return nil, nil
	}

	path := GetScratchVolumePath(scratch)
	volumeSource := corev1.VolumeSource{}

	switch {
	case scratch != nil && scratch.Ephemeral != nil:
		volumeSource.Ephemeral = scratch.Ephemeral
	case scratch != nil:
		volumeSource.EmptyDir = &corev1.EmptyDirVolumeSource{
			Medium:    scratch.Medium,
			SizeLimit: scratch.SizeLimit,
		}
	default:
		volumeSource.EmptyDir = &corev1.EmptyDirVolumeSource{}
	}

	volumes := []containers.VolumeMutator{
		func(v *[]corev1.Volume) {
			*v = append(*v, corev1.Volume{
				Name:         VolumeRenovateTmp,
				VolumeSource: volumeSource,
			})
		},
	}

	mounts := []corev1.VolumeMount{
		{
			Name:      VolumeRenovateTmp,
			MountPath: path,
		},
	}

	return volumes, mounts
}

// GetScratchVolumePath returns the effective mount path for the scratch volume.
func GetScratchVolumePath(scratch *renovatev1beta1.ScratchVolumeSpec) string {
	if scratch != nil && scratch.Path != "" {
		return scratch.Path
	}

	return renovatev1beta1.DefaultScratchVolumePath
}
