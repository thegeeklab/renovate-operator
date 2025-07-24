package runner

import (
	"context"
	"fmt"

	"github.com/thegeeklab/renovate-operator/pkg/equality"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/renovate"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *runnerReconciler) reconcileCronJob(ctx context.Context) (*ctrl.Result, error) {
	// Only create CronJob if schedule is specified
	if r.instance.Spec.Schedule == "" {
		// If no schedule, return without creating CronJob
		return &ctrl.Result{}, nil
	}

	expected, err := r.createCronJob()
	if err != nil {
		return &ctrl.Result{}, err
	}

	return r.ReconcileResource(ctx, &batchv1.CronJob{}, expected, equality.CronJobEqual)
}

func (r *runnerReconciler) createCronJob() (*batchv1.CronJob, error) {
	cronJob := &batchv1.CronJob{
		ObjectMeta: metadata.RunnerMetaData(r.Req),
		Spec: batchv1.CronJobSpec{
			Schedule:          r.instance.Spec.Schedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			Suspend:           r.instance.Spec.Suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: r.createJobSpecForCronJob(),
			},
		},
	}

	if err := controllerutil.SetControllerReference(r.instance, cronJob, r.Scheme); err != nil {
		return nil, err
	}

	return cronJob, nil
}

func (r *runnerReconciler) createJobSpecForCronJob() batchv1.JobSpec {
	// Create a Job that will create RenovatorJobs for all batches
	// The Job will be created by the CronJob on schedule
	return batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "renovate-operator",
					"app.kubernetes.io/name":       "renovator-scheduled-runner",
					"renovator.renovate/name":      r.instance.Name,
				},
			},
			Spec: r.createScheduledPodSpec(),
		},
	}
}

func (r *runnerReconciler) createScheduledPodSpec() corev1.PodSpec {
	// Create pod spec that will create RenovatorJobs for all batches
	return corev1.PodSpec{
		ServiceAccountName: metadata.GenericMetaData(r.Req).Name,
		ImagePullSecrets:   r.instance.Spec.ImagePullSecrets,
		RestartPolicy:      corev1.RestartPolicyNever,
		Volumes: append(
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
			}),
		Containers: []corev1.Container{
			{
				Name:            "renovate-job-scheduler",
				Image:           r.instance.Spec.Image,
				Command:         []string{"/job-scheduler"},
				ImagePullPolicy: r.instance.Spec.ImagePullPolicy,
				Env: []corev1.EnvVar{
					{
						Name:  "RENOVATOR_NAME",
						Value: r.instance.Name,
					},
					{
						Name:  "RENOVATOR_NAMESPACE",
						Value: r.instance.Namespace,
					},
					{
						Name:  "BATCH_CONFIG_FILE",
						Value: renovate.FileRenovateBatches,
					},
					{
						Name:  "MAX_PARALLEL_JOBS",
						Value: fmt.Sprintf("%d", r.instance.Spec.Runner.Instances),
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      renovate.VolumeRenovateTmp,
						ReadOnly:  true,
						MountPath: renovate.DirRenovateTmp,
					},
				},
			},
		},
	}
}
