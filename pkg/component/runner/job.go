package runner

import (
	"context"
	"fmt"
	"math"

	"github.com/thegeeklab/renovate-operator/pkg/dispatcher"
	"github.com/thegeeklab/renovate-operator/pkg/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var ErrMaxBatchCount = fmt.Errorf("max batch count reached")

func (r *Reconciler) reconcileCronJob(ctx context.Context) (*ctrl.Result, error) {
	job := &batchv1.CronJob{ObjectMeta: RunnerMetaData(r.req)}

	_, err := k8s.CreateOrUpdate(ctx, r.Client, job, r.instance, func() error {
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

	if err := r.updateJobSpec(&job.Spec.JobTemplate.Spec); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) updateJobSpec(spec *batchv1.JobSpec) error {
	batchCount := len(r.batches)
	if batchCount > math.MaxInt32 {
		return fmt.Errorf("%w: %d", ErrMaxBatchCount, batchCount)
	}

	spec.CompletionMode = ptr.To(batchv1.IndexedCompletion)
	spec.Completions = ptr.To(int32(batchCount))
	spec.Parallelism = ptr.To(r.instance.Spec.Runner.Instances)
	spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	spec.Template.Spec.Volumes = append(
		renovate.DefaultVolume(corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}),
		corev1.Volume{
			Name: renovate.VolumeRenovateTmp,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: r.instance.Name,
					},
				},
			},
		},
		corev1.Volume{
			Name: renovate.VolumeRenovateBase,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})

	spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name:            "renovate-dispatcher",
			Image:           r.instance.Spec.Image,
			Command:         []string{"/dispatcher"},
			ImagePullPolicy: r.instance.Spec.ImagePullPolicy,
			Env: []corev1.EnvVar{
				{
					Name:  dispatcher.EnvRenovateRawConfig,
					Value: renovate.FileRenovateTmp,
				},
				{
					Name:  dispatcher.EnvRenovateConfig,
					Value: renovate.FileRenovateConfig,
				},
				{
					Name:  dispatcher.EnvRenovateBatches,
					Value: renovate.FileRenovateBatches,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      renovate.VolumeRenovateConfig,
					MountPath: renovate.DirRenovateConfig,
				},
				{
					Name:      renovate.VolumeRenovateTmp,
					ReadOnly:  true,
					MountPath: renovate.DirRenovateTmp,
				},
			},
		},
	}
	spec.Template.Spec.Containers = []corev1.Container{
		renovate.DefaultContainer(r.instance, []corev1.EnvVar{}, []string{}),
	}

	return nil
}
