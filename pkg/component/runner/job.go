package runner

import (
	"context"
	"fmt"
	"math"

	"github.com/thegeeklab/renovate-operator/pkg/dispatcher"
	"github.com/thegeeklab/renovate-operator/pkg/renovate"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var ErrMaxBatchCount = fmt.Errorf("max batch count reached")

func (r *RunnerReconciler) reconcileCronJob(ctx context.Context) (*ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)

	spec, err := r.createJobSpec(r.Batches)
	if err != nil {
		return &ctrl.Result{}, err
	}

	obj, err := r.createCronJob(spec)
	if err != nil {
		return &ctrl.Result{}, err
	}

	op, err := ctrl.CreateOrUpdate(ctx, r.Client, obj, nil)
	if err != nil {
		return &ctrl.Result{}, err
	}

	ctxLogger.V(1).Info("Runner CronJob", "object", client.ObjectKeyFromObject(obj), "operation", op)

	return &ctrl.Result{}, nil
}

func (r *RunnerReconciler) createCronJob(spec batchv1.JobSpec) (*batchv1.CronJob, error) {
	cronJob := &batchv1.CronJob{
		ObjectMeta: RunnerMetaData(r.Req),
		Spec: batchv1.CronJobSpec{
			Schedule:          r.Instance.Spec.Schedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			Suspend:           r.Instance.Spec.Suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: spec,
			},
		},
	}

	if err := controllerutil.SetControllerReference(r.Instance, cronJob, r.Scheme); err != nil {
		return nil, err
	}

	return cronJob, nil
}

func (r *RunnerReconciler) createJobSpec(batches []Batch) (batchv1.JobSpec, error) {
	batchCount := len(batches)
	if batchCount > math.MaxInt32 {
		return batchv1.JobSpec{}, fmt.Errorf("%w: %d", ErrMaxBatchCount, batchCount)
	}

	completionMode := batchv1.IndexedCompletion
	completions := int32(batchCount)
	parallelism := r.Instance.Spec.Runner.Instances

	return batchv1.JobSpec{
		CompletionMode: &completionMode,
		Completions:    &completions,
		Parallelism:    &parallelism,
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Volumes: append(
					renovate.DefaultVolume(corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					}),
					corev1.Volume{
						Name: renovate.VolumeRenovateTmp,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: r.Instance.Name,
								},
							},
						},
					},
					corev1.Volume{
						Name: renovate.VolumeRenovateBase,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					}),
				InitContainers: []corev1.Container{
					{
						Name:            "renovate-dispatcher",
						Image:           r.Instance.Spec.Image,
						Command:         []string{"/dispatcher"},
						ImagePullPolicy: r.Instance.Spec.ImagePullPolicy,
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
				},
				Containers: []corev1.Container{
					renovate.DefaultContainer(r.Instance, []corev1.EnvVar{}, []string{}),
				},
			},
		},
	}, nil
}
