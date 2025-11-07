package discovery

import (
	"context"
	"strings"

	"github.com/thegeeklab/renovate-operator/pkg/discovery"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/renovate"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *discoveryReconciler) reconcileCronJob(ctx context.Context) (*ctrl.Result, error) {
	expected, err := r.createCronJob()
	if err != nil {
		return &ctrl.Result{}, err
	}

	return r.ReconcileResource(ctx, &batchv1.CronJob{}, expected)
}

func (r *discoveryReconciler) createCronJob() (*batchv1.CronJob, error) {
	cronJob := &batchv1.CronJob{
		ObjectMeta: metadata.DiscoveryMetaData(r.Req),
		Spec: batchv1.CronJobSpec{
			Schedule:          r.instance.Spec.Discovery.Schedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			Suspend:           r.instance.Spec.Discovery.Suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: r.createJobSpec(),
			},
		},
	}

	if err := controllerutil.SetControllerReference(r.instance, cronJob, r.Scheme); err != nil {
		return nil, err
	}

	return cronJob, nil
}

func (r *discoveryReconciler) createJobSpec() batchv1.JobSpec {
	return batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				ServiceAccountName: metadata.GenericMetaData(r.Req).Name,
				RestartPolicy:      corev1.RestartPolicyNever,
				Volumes: append(
					renovate.DefaultVolume(corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: r.instance.Name,
							},
						},
					}),
					corev1.Volume{
						Name: renovate.VolumeRenovateBase,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					}),
				InitContainers: []corev1.Container{
					renovate.DefaultContainer(r.instance, []corev1.EnvVar{
						{
							Name:  "RENOVATE_AUTODISCOVER",
							Value: "true",
						},
						{
							Name:  "RENOVATE_AUTODISCOVER_FILTER",
							Value: strings.Join(r.instance.Spec.Discovery.Filter, ","),
						},
					}, []string{"--write-discovered-repos", renovate.FileRenovateRepositories}),
				},
				Containers: []corev1.Container{
					{
						Name:            "renovate-discovery",
						Image:           r.instance.Spec.Image,
						Command:         []string{"/discovery"},
						ImagePullPolicy: r.instance.Spec.ImagePullPolicy,
						Env: []corev1.EnvVar{
							{
								Name:  discovery.EnvRenovatorInstanceName,
								Value: r.instance.Name,
							},
							{
								Name:  discovery.EnvRenovatorInstanceNamespace,
								Value: r.instance.Namespace,
							},
							{
								Name:  discovery.EnvRenovateOutputFile,
								Value: renovate.FileRenovateRepositories,
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      renovate.VolumeRenovateBase,
								MountPath: renovate.DirRenovateTmp,
							},
						},
					},
				},
			},
		},
	}
}
