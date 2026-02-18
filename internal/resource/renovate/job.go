package renovate

import (
	"context"
	"sort"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// jobConfig holds the state required to build the JobSpec.
type jobConfig struct {
	// Context
	Renovate   *renovatev1beta1.RenovateConfig
	RenovateCM string

	// Accumulators
	InitContainers []corev1.Container
	VolumeMutators []containers.VolumeMutator
	EnvVars        []corev1.EnvVar
}

// JobOption defines a function that modifies the job configuration.
type JobOption func(*jobConfig)

// DefaultJobSpec applies the Renovate job specification.
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
	spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	spec.Template.Spec.InitContainers = cfg.InitContainers

	// Build Final Volume Slice
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

// GetActiveJobs returns a list of running jobs matching the given labels.
func GetActiveJobs(
	ctx context.Context,
	c client.Client,
	namespace string,
	labels map[string]string,
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

// PruneJobHistory deletes old completed/failed jobs based on the provided limits.
func PruneJobHistory(
	ctx context.Context,
	c client.Client,
	namespace string,
	labels map[string]string,
	successLimit int,
	failedLimit int,
) error {
	jobList := &batchv1.JobList{}

	err := c.List(ctx, jobList,
		client.InNamespace(namespace),
		client.MatchingLabels(labels),
	)
	if err != nil {
		return err
	}

	var successful, failed []batchv1.Job

	for _, job := range jobList.Items {
		if job.Status.Active > 0 {
			continue
		}

		if job.Status.Succeeded > 0 {
			successful = append(successful, job)
		} else if job.Status.Failed > 0 {
			failed = append(failed, job)
		}
	}

	deleteOldestJobs(ctx, c, successful, successLimit)
	deleteOldestJobs(ctx, c, failed, failedLimit)

	return nil
}

// deleteOldestJobs removes jobs that exceed the count limit, starting with the oldest.
func deleteOldestJobs(ctx context.Context, c client.Client, jobs []batchv1.Job, limit int) {
	if len(jobs) <= limit {
		return
	}

	// Sort by creation timestamp (Oldest first)
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreationTimestamp.Before(&jobs[j].CreationTimestamp)
	})

	// Delete excess jobs
	for i := 0; i < len(jobs)-limit; i++ {
		_ = c.Delete(ctx, &jobs[i], client.PropagationPolicy(metav1.DeletePropagationBackground))
	}
}
