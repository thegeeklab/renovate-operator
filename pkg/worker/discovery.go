package worker

import (
	"context"
	"strings"

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

func (w *Worker) reconcileDiscovery(ctx context.Context) (*ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)

	// Cronjob
	jobSpec := w.createDiscoveryJobSpec()

	expectedCronJob, cjCreationErr := w.createDiscoveryCronJob(jobSpec)
	if cjCreationErr != nil {
		return &ctrl.Result{}, cjCreationErr
	}

	ctxLogger = ctxLogger.WithValues("cronJob", expectedCronJob.Name)
	key := types.NamespacedName{
		Namespace: expectedCronJob.Namespace,
		Name:      expectedCronJob.Name,
	}

	currentObject := batchv1.CronJob{}

	err := w.client.Get(ctx, key, &currentObject)
	if err != nil {
		if errors.IsNotFound(err) {
			if err = w.client.Create(ctx, expectedCronJob); err != nil {
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

		err := w.client.Update(ctx, expectedCronJob)
		if err != nil {
			ctxLogger.Error(err, "Failed to update CronJob")

			return &ctrl.Result{}, err
		}

		ctxLogger.Info("Updated CronJob")

		return &ctrl.Result{Requeue: true}, nil
	}

	return &ctrl.Result{}, nil
}

func (w *Worker) createDiscoveryCronJob(jobSpec batchv1.JobSpec) (*batchv1.CronJob, error) {
	cronJob := batchv1.CronJob{
		ObjectMeta: metadata.DiscoveryMetaData(w.req),
		Spec: batchv1.CronJobSpec{
			// Schedule:          w.discoveryRes.Spec.Discovery.Schedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			// Suspend:           w.discoveryRes.Spec.Suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: jobSpec,
			},
		},
	}

	if err := controllerutil.SetControllerReference(w.instance, &cronJob, w.scheme); err != nil {
		return nil, err
	}

	return &cronJob, nil
}

func (w *Worker) createDiscoveryJobSpec() batchv1.JobSpec {
	return batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				ServiceAccountName: metadata.GenericMetaData(w.req).Name,
				RestartPolicy:      corev1.RestartPolicyNever,
				Volumes: renovate.StandardVolumes(corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: w.instance.Name,
						},
					},
				}),
				InitContainers: []corev1.Container{
					renovate.Container(w.instance, []corev1.EnvVar{
						{
							Name:  "RENOVATE_AUTODISCOVER",
							Value: "true",
						},
						{
							Name:  "RENOVATE_AUTODISCOVER_FILTER",
							Value: strings.Join(w.instance.Spec.Discovery.Filter, ","),
						},
					}, []string{"--write-discovered-repos", renovate.FileRenovateConfigOutput}),
				},
				Containers: []corev1.Container{
					{
						Name:            "renovate-discovery",
						Image:           w.instance.Spec.Image,
						ImagePullPolicy: w.instance.Spec.ImagePullPolicy,
						Env: []corev1.EnvVar{
							{
								Name:  discovery.EnvRenovatorInstanceName,
								Value: w.instance.Name,
							},
							{
								Name:  discovery.EnvRenovatorInstanceNamespace,
								Value: w.instance.Namespace,
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
