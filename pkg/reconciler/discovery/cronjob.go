package discovery

import (
	"context"
	"strings"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/discovery"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/renovate"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *discoveryReconciler) reconcileCronJob(ctx context.Context) (*ctrl.Result, error) {
	// Check if immediate discovery is requested via annotation
	if r.isDiscoverOperationRequested() {
		return r.handleImmediateDiscovery(ctx)
	}

	return r.handleScheduledDiscovery(ctx)
}

func (r *discoveryReconciler) isDiscoverOperationRequested() bool {
	val, ok := r.instance.Annotations[renovatev1beta1.AnnotationOperation]

	return ok && strings.EqualFold(val, string(renovatev1beta1.OperationDiscover))
}

func (r *discoveryReconciler) handleImmediateDiscovery(ctx context.Context) (*ctrl.Result, error) {
	// Check if there's already a discovery job running
	hasRunningJob, err := r.hasRunningDiscoveryJob(ctx)
	if err != nil {
		return &ctrl.Result{}, err
	}

	if hasRunningJob {
		// Skip creating a new job if one is already running
		return &ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Create a one-time job for immediate discovery
	job, err := r.createDiscoveryJob()
	if err != nil {
		return &ctrl.Result{}, err
	}

	// Remove the annotation to prevent repeated immediate discoveries
	if err := r.removeDiscoveryAnnotation(ctx); err != nil {
		return &ctrl.Result{}, err
	}

	return r.ReconcileResource(ctx, &batchv1.Job{}, job)
}

func (r *discoveryReconciler) handleScheduledDiscovery(ctx context.Context) (*ctrl.Result, error) {
	// Normal scheduled discovery
	expected, err := r.createCronJob()
	if err != nil {
		return &ctrl.Result{}, err
	}

	return r.ReconcileResource(ctx, &batchv1.CronJob{}, expected)
}

func (r *discoveryReconciler) hasRunningDiscoveryJob(ctx context.Context) (bool, error) {
	// Check for active discovery jobs
	existingJobs := &batchv1.JobList{}
	if err := r.KubeClient.List(ctx, existingJobs, client.InNamespace(r.instance.Namespace)); err != nil {
		return false, err
	}

	discoveryName := metadata.DiscoveryName(r.Req)
	for _, job := range existingJobs.Items {
		// Check both manually created discovery jobs and jobs created by CronJob
		if job.Name == discoveryName || strings.HasPrefix(job.Name, discoveryName) {
			if job.Status.Active > 0 {
				return true, nil
			}
		}
	}

	return false, nil
}

func (r *discoveryReconciler) createDiscoveryJob() (*batchv1.Job, error) {
	job := &batchv1.Job{
		ObjectMeta: metadata.DiscoveryMetaData(r.Req),
		Spec:       r.createJobSpec(),
	}

	if err := controllerutil.SetControllerReference(r.instance, job, r.Scheme); err != nil {
		return nil, err
	}

	return job, nil
}

func (r *discoveryReconciler) removeDiscoveryAnnotation(ctx context.Context) error {
	// Remove the annotation to prevent repeated immediate discoveries
	if r.instance.Annotations == nil {
		r.instance.Annotations = make(map[string]string)
	}

	delete(r.instance.Annotations, renovatev1beta1.AnnotationOperation)

	return r.KubeClient.Update(ctx, r.instance)
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
