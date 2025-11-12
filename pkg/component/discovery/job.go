package discovery

import (
	"context"
	"strings"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/discovery"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *DiscoveryReconciler) reconcileCronJob(ctx context.Context) (*ctrl.Result, error) {
	// Check if immediate discovery is requested via annotation
	if r.isDiscoverOperationRequested() {
		return r.handleImmediateDiscovery(ctx)
	}

	return r.handleScheduledDiscovery(ctx)
}

func (r *DiscoveryReconciler) isDiscoverOperationRequested() bool {
	val, ok := r.Instance.Annotations[renovatev1beta1.AnnotationOperation]

	return ok && strings.EqualFold(val, string(renovatev1beta1.OperationDiscover))
}

func (r *DiscoveryReconciler) handleImmediateDiscovery(ctx context.Context) (*ctrl.Result, error) {
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
	obj, err := r.createDiscoveryJob()
	if err != nil {
		return &ctrl.Result{}, err
	}

	// Remove the annotation to prevent repeated immediate discoveries
	if err := r.removeDiscoveryAnnotation(ctx); err != nil {
		return &ctrl.Result{}, err
	}

	_, err = k8s.CreateOrUpdate(ctx, r.Client, obj, r.Instance, nil)
	if err != nil {
		return &ctrl.Result{}, err
	}

	return &ctrl.Result{}, nil
}

func (r *DiscoveryReconciler) handleScheduledDiscovery(ctx context.Context) (*ctrl.Result, error) {
	obj := &batchv1.CronJob{ObjectMeta: DiscoveryMetaData(r.Req)}

	op, err := k8s.CreateOrUpdate(ctx, r.Client, obj, r.Instance, func() error {
		r.updateCronJob(obj)

		return nil
	})
	if err != nil {
		return &ctrl.Result{}, err
	}

	if op == controllerutil.OperationResultUpdated {
		if err := r.DeleteAllOf(ctx, &batchv1.Job{},
			client.InNamespace(r.Req.Namespace),
			client.MatchingLabels(DiscoveryJobLabels()),
			client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
			return &ctrl.Result{}, err
		}
	}

	return &ctrl.Result{}, nil
}

func (r *DiscoveryReconciler) hasRunningDiscoveryJob(ctx context.Context) (bool, error) {
	// Check for active discovery jobs
	existingJobs := &batchv1.JobList{}
	if err := r.List(ctx, existingJobs, client.InNamespace(r.Instance.Namespace)); err != nil {
		return false, err
	}

	discoveryName := DiscoveryName(r.Req)
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

func (r *DiscoveryReconciler) createDiscoveryJob() (*batchv1.Job, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: DiscoveryName(r.Req) + "-",
			Namespace:    r.Instance.Namespace,
		},
		Spec: r.createJobSpec(),
	}

	if err := controllerutil.SetControllerReference(r.Instance, job, r.Scheme); err != nil {
		return nil, err
	}

	return job, nil
}

func (r *DiscoveryReconciler) removeDiscoveryAnnotation(ctx context.Context) error {
	// Remove the annotation to prevent repeated immediate discoveries
	if r.Instance.Annotations == nil {
		r.Instance.Annotations = make(map[string]string)
	}

	delete(r.Instance.Annotations, renovatev1beta1.AnnotationOperation)

	return r.Update(ctx, r.Instance)
}

func (r *DiscoveryReconciler) updateCronJob(job *batchv1.CronJob) *batchv1.CronJob {
	job.Spec.Schedule = r.Instance.Spec.Discovery.Schedule
	job.Spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
	job.Spec.Suspend = r.Instance.Spec.Discovery.Suspend
	job.Spec.JobTemplate = batchv1.JobTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: DiscoveryJobLabels(),
		},
		Spec: r.createJobSpec(),
	}

	return job
}

func (r *DiscoveryReconciler) createJobSpec() batchv1.JobSpec {
	return batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				ServiceAccountName: metadata.GenericMetaData(r.Req).Name,
				RestartPolicy:      corev1.RestartPolicyNever,
				Volumes: append(
					renovate.DefaultVolume(corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: r.Instance.Name,
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
					renovate.DefaultContainer(r.Instance, []corev1.EnvVar{
						{
							Name:  "RENOVATE_AUTODISCOVER",
							Value: "true",
						},
						{
							Name:  "RENOVATE_AUTODISCOVER_FILTER",
							Value: strings.Join(r.Instance.Spec.Discovery.Filter, ","),
						},
					}, []string{"--write-discovered-repos", renovate.FileRenovateRepositories}),
				},
				Containers: []corev1.Container{
					{
						Name:            "renovate-discovery",
						Image:           r.Instance.Spec.Image,
						Command:         []string{"/discovery"},
						ImagePullPolicy: r.Instance.Spec.ImagePullPolicy,
						Env: []corev1.EnvVar{
							{
								Name:  discovery.EnvRenovatorInstanceName,
								Value: r.Instance.Name,
							},
							{
								Name:  discovery.EnvRenovatorInstanceNamespace,
								Value: r.Instance.Namespace,
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

func DiscoveryJobLabels() map[string]string {
	return map[string]string{
		"renovate.thegeeklab.de/job-type": "cron",
	}
}
