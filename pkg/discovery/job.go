package discovery

import (
	"context"

	"github.com/thegeeklab/renovate-operator/discovery"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/renovate"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (d *Discovery) reconcileDiscovery(ctx context.Context) (*ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)

	// Cronjob
	jobSpec := d.createDiscoveryJobSpec()

	expectedCronJob, cjCreationErr := d.createDiscoveryCronJob(jobSpec)
	if cjCreationErr != nil {
		return &ctrl.Result{}, cjCreationErr
	}

	ctxLogger = ctxLogger.WithValues("cronJob", expectedCronJob.Name)
	key := types.NamespacedName{
		Namespace: expectedCronJob.Namespace,
		Name:      expectedCronJob.Name,
	}

	currentObject := batchv1.CronJob{}

	err := d.client.Get(ctx, key, &currentObject)
	if err != nil {
		if errors.IsNotFound(err) {
			if err = d.client.Create(ctx, expectedCronJob); err != nil {
				ctxLogger.Error(err, "Failed to create Cronjob")

				return &ctrl.Result{}, err
			}

			ctxLogger.Info("Created Cronjob")

			return &ctrl.Result{Requeue: true}, nil
		}

		return &ctrl.Result{}, err
	}

	if !equality.Semantic.DeepEqual(*expectedCronJob, currentObject) {
		ctxLogger.Info("Updating CronJob")

		err := d.client.Update(ctx, expectedCronJob)
		if err != nil {
			ctxLogger.Error(err, "Failed to update CronJob")

			return &ctrl.Result{}, err
		}

		ctxLogger.Info("Updated CronJob")

		return &ctrl.Result{Requeue: true}, nil
	}

	return &ctrl.Result{}, nil
}

func (d *Discovery) createDiscoveryCronJob(jobSpec batchv1.JobSpec) (*batchv1.CronJob, error) {
	cronJob := batchv1.CronJob{
		ObjectMeta: metadata.DiscoveryMetaData(d.req),
		Spec: batchv1.CronJobSpec{
			// Schedule:          d.instance.Spec.Discovery.Schedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			Suspend:           d.instance.Spec.Suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: jobSpec,
			},
		},
	}

	if err := controllerutil.SetControllerReference(&d.instance, &cronJob, d.scheme); err != nil {
		return nil, err
	}

	return &cronJob, nil
}

func (d *Discovery) createDiscoveryJobSpec() batchv1.JobSpec {
	return batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				ServiceAccountName: metadata.GenericMetaData(d.req).Name,
				RestartPolicy:      corev1.RestartPolicyNever,
				Volumes: renovate.StandardVolumes(corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: d.instance.Name,
						},
					},
				}),
				InitContainers: []corev1.Container{
					renovate.Container(d.instance, []corev1.EnvVar{
						{
							Name:  "RENOVATE_AUTODISCOVER",
							Value: "true",
						},
					}, []string{"--write-discovered-repos", renovate.FileRenovateConfigOutput}),
				},
				Containers: []corev1.Container{
					{
						Name:  "renovate-discovery",
						Image: "quay.io/thegeeklab/renovate-discovery:0.1.0", // TODO allow overwrite
						Env: []corev1.EnvVar{
							{
								Name:  discovery.EnvRenovateCrName,
								Value: d.instance.Name,
							},
							{
								Name:  discovery.EnvRenovateCrNamespace,
								Value: d.instance.Namespace,
							},
							{
								Name:  discovery.EnvRenovateOutputFile,
								Value: renovate.FileRenovateConfigOutput,
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      renovate.VolumeWorkDir,
								MountPath: renovate.DirRenovateBase,
							},
						},
					},
				},
			},
		},
	}
}
