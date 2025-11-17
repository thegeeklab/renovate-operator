package discovery

import (
	"context"
	"fmt"
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
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// RequeueDelay is the default delay when requeuing operations.
	RequeueDelay = time.Minute
)

func (r *Reconciler) reconcileCronJob(ctx context.Context) (*ctrl.Result, error) {
	// Check if immediate discovery is requested via annotation
	annotationValue, hasAnnotation := r.instance.Annotations[renovatev1beta1.AnnotationOperation]
	if hasAnnotation && strings.EqualFold(annotationValue, string(renovatev1beta1.OperationDiscover)) {
		return r.handleBatchJob(ctx)
	}

	return r.handleCronJob(ctx)
}

func (r *Reconciler) handleBatchJob(ctx context.Context) (*ctrl.Result, error) {
	hasRunningJob, err := r.hasRunningJob(ctx)
	if err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to check running jobs: %w", err)
	}

	if hasRunningJob {
		return &ctrl.Result{RequeueAfter: RequeueDelay}, nil
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: DiscoveryName(r.req) + "-",
			Namespace:    r.instance.Namespace,
			Labels:       DiscoveryJobLabels(),
		},
		Spec: batchv1.JobSpec{},
	}

	r.updateJobSpec(&job.Spec)

	_, err = k8s.CreateOrPatch(ctx, r.Client, job, r.instance, nil)
	if err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to create or update job: %w", err)
	}

	if err := r.removeDiscoveryAnnotation(ctx); err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to remove discovery annotation: %w", err)
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) handleCronJob(ctx context.Context) (*ctrl.Result, error) {
	job := &batchv1.CronJob{ObjectMeta: DiscoveryMetaData(r.req)}

	op, err := k8s.CreateOrPatch(ctx, r.Client, job, r.instance, func() error {
		return r.updateCronJob(job)
	})
	if err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to create or update cron job: %w", err)
	}

	if op == controllerutil.OperationResultUpdated {
		// Clean up old jobs when CronJob is updated
		if err := r.DeleteAllOf(ctx, &batchv1.Job{},
			client.InNamespace(r.req.Namespace),
			client.MatchingLabels(DiscoveryJobLabels()),
			client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
			return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to delete old jobs: %w", err)
		}
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) hasRunningJob(ctx context.Context) (bool, error) {
	// Check for active discovery jobs with our specific labels
	existingJobs := &batchv1.JobList{}
	if err := r.List(ctx, existingJobs,
		client.InNamespace(r.instance.Namespace),
		client.MatchingLabels(DiscoveryJobLabels())); err != nil {
		return false, fmt.Errorf("failed to list existing jobs: %w", err)
	}

	discoveryName := DiscoveryName(r.req)
	for _, job := range existingJobs.Items {
		// Check both manually created discovery jobs and jobs created by CronJob
		if job.Name == discoveryName || strings.HasPrefix(job.Name, discoveryName+"-") {
			// Consider job as running if it's active or hasn't completed yet
			if job.Status.Active > 0 || (job.Status.Succeeded == 0 && job.Status.Failed == 0) {
				return true, nil
			}
		}
	}

	return false, nil
}

func (r *Reconciler) removeDiscoveryAnnotation(ctx context.Context) error {
	if r.instance.Annotations == nil {
		r.instance.Annotations = make(map[string]string)
	}

	delete(r.instance.Annotations, renovatev1beta1.AnnotationOperation)

	if err := r.Update(ctx, r.instance); err != nil {
		return fmt.Errorf("failed to update instance after removing annotation: %w", err)
	}

	return nil
}

func (r *Reconciler) updateCronJob(job *batchv1.CronJob) error {
	job.Spec.Schedule = r.instance.Spec.Discovery.Schedule
	job.Spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
	job.Spec.Suspend = r.instance.Spec.Discovery.Suspend
	job.Spec.JobTemplate.Labels = DiscoveryJobLabels()

	r.updateJobSpec(&job.Spec.JobTemplate.Spec)

	return nil
}

func (r *Reconciler) updateJobSpec(spec *batchv1.JobSpec) {
	spec.Template.Spec.ServiceAccountName = metadata.GenericMetaData(r.req).Name
	spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	r.setJobVolumes(&spec.Template.Spec)
	r.setInitContainers(&spec.Template.Spec)
	r.setMainContainer(&spec.Template.Spec)
}

func (r *Reconciler) setJobVolumes(podSpec *corev1.PodSpec) {
	podSpec.Volumes = []corev1.Volume{
		{
			Name: renovate.VolumeRenovateConfig,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: ptr.To(corev1.ConfigMapVolumeSourceDefaultMode),
					LocalObjectReference: corev1.LocalObjectReference{
						Name: r.instance.Name,
					},
				},
			},
		},
		{
			Name: renovate.VolumeRenovateBase,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
}

func (r *Reconciler) setInitContainers(podSpec *corev1.PodSpec) {
	podSpec.InitContainers = []corev1.Container{
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
	}
}

func (r *Reconciler) setMainContainer(podSpec *corev1.PodSpec) {
	podSpec.Containers = []corev1.Container{
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
	}
}
