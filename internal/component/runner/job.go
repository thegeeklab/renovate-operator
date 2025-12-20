package runner

import (
	"context"
	"errors"
	"fmt"
	"math"

	renovateconfig "github.com/thegeeklab/renovate-operator/internal/component/renovate-config"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var ErrMaxBatchCount = errors.New("max batch count reached")

func (r *Reconciler) reconcileCronJob(ctx context.Context) (*ctrl.Result, error) {
	job := &batchv1.CronJob{ObjectMeta: RunnerMetadata(r.req)}

	_, err := k8s.CreateOrPatch(ctx, r.Client, job, r.instance, func() error {
		return r.updateCronJob(job)
	})
	if err != nil {
		return &ctrl.Result{}, err
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateCronJob(job *batchv1.CronJob) error {
	job.Spec.Schedule = r.instance.Spec.Schedule
	job.Spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
	job.Spec.Suspend = r.instance.Spec.Suspend

	return r.updateJobSpec(&job.Spec.JobTemplate.Spec)
}

func (r *Reconciler) updateJobSpec(spec *batchv1.JobSpec) error {
	batchCount := len(r.batches)
	if batchCount > math.MaxInt32 {
		return fmt.Errorf("%w: %d", ErrMaxBatchCount, batchCount)
	}

	renovateConfigCM := metadata.GenericName(r.req, renovateconfig.ConfigMapSuffix)
	renovateBatchesCM := metadata.GenericName(r.req, ConfigMapSuffix)

	spec.CompletionMode = ptr.To(batchv1.IndexedCompletion)
	spec.Completions = ptr.To(int32(batchCount))
	spec.Parallelism = ptr.To(r.instance.Spec.Instances)
	spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	spec.Template.Spec.Volumes = containers.VolumesTemplate(
		containers.WithEmptyDirVolume(renovate.VolumeRenovateConfig),
		containers.WithConfigMapVolume(renovateConfigCM, renovateConfigCM),
		containers.WithConfigMapVolume(renovateBatchesCM, renovateBatchesCM),
	)

	spec.Template.Spec.InitContainers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate-dispatcher",
			r.instance.Spec.Image,
			r.instance.Spec.ImagePullPolicy,
			containers.WithEnvVars([]corev1.EnvVar{
				{
					Name:  renovate.EnvRenovateConfigRaw,
					Value: renovate.FileRenovateTmp,
				},
				{
					Name:  renovate.EnvRenovateConfig,
					Value: renovate.FileRenovateConfig,
				},
				{
					Name:  renovate.EnvRenovateBatches,
					Value: renovate.FileRenovateBatches,
				},
			}),
			containers.WithContainerCommand([]string{"/dispatcher"}),
			containers.WithVolumeMounts([]corev1.VolumeMount{
				{
					Name:      renovate.VolumeRenovateConfig,
					MountPath: renovate.DirRenovateConfig,
				},
				{
					Name:      renovateConfigCM,
					ReadOnly:  true,
					MountPath: renovate.FileRenovateTmp,
					SubPath:   renovate.FilenameRenovateConfig,
				},
				{
					Name:      renovateBatchesCM,
					ReadOnly:  true,
					MountPath: renovate.FileRenovateBatches,
					SubPath:   renovate.FilenameBatches,
				},
			}),
		),
	}
	spec.Template.Spec.Containers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate",
			r.renovate.Spec.Image,
			r.renovate.Spec.ImagePullPolicy,
			containers.WithEnvVars(renovate.DefaultEnvVars(&r.renovate.Spec)),
			containers.WithVolumeMounts(
				[]corev1.VolumeMount{
					{
						Name:      renovate.VolumeRenovateConfig,
						MountPath: renovate.DirRenovateConfig,
					},
				},
			),
		),
	}

	return nil
}
