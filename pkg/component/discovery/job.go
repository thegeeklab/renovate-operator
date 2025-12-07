package discovery

import (
	"context"
	"strings"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	"github.com/thegeeklab/renovate-operator/internal/resource/cronjob"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	renovateconfig "github.com/thegeeklab/renovate-operator/pkg/component/renovate-config"
	"github.com/thegeeklab/renovate-operator/pkg/discovery"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	if HasRenovatorOperationDiscover(r.instance) {
		return r.handleImmediateDiscovery(ctx)
	}

	job := &batchv1.CronJob{ObjectMeta: DiscoveryMetadata(r.req)}

	op, err := k8s.CreateOrPatch(ctx, r.Client, job, r.instance, func() error {
		return r.updateCronJob(job)
	})
	if err != nil {
		return &ctrl.Result{}, err
	}

	if op == controllerutil.OperationResultUpdated {
		if err := cronjob.DeleteOwnedJobs(ctx, r.Client, job); err != nil {
			return &ctrl.Result{}, err
		}
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) handleImmediateDiscovery(ctx context.Context) (*ctrl.Result, error) {
	// Check for active discovery jobs with our specific labels
	existingJobs := &batchv1.JobList{}
	if err := r.List(ctx, existingJobs, client.InNamespace(r.instance.Namespace)); err != nil {
		return &ctrl.Result{}, err
	}

	discoveryName := DiscoveryName(r.req)
	for _, job := range existingJobs.Items {
		// Check both manually created discovery jobs and jobs created by CronJob
		if job.Name == discoveryName || strings.HasPrefix(job.Name, discoveryName+"-") {
			// Consider job as running if it's active or hasn't completed yet
			if job.Status.Active > 0 || (job.Status.Succeeded == 0 && job.Status.Failed == 0) {
				return &ctrl.Result{RequeueAfter: RequeueDelay}, nil
			}
		}
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: DiscoveryName(r.req) + "-",
			Namespace:    r.instance.Namespace,
		},
		Spec: batchv1.JobSpec{},
	}

	r.updateJobSpec(&job.Spec)

	_, err := k8s.CreateOrPatch(ctx, r.Client, job, r.instance, nil)
	if err != nil {
		return &ctrl.Result{}, err
	}

	// Remove discovery annotation
	if r.instance.Annotations == nil {
		r.instance.Annotations = make(map[string]string)
	}

	delete(r.instance.Annotations, renovatev1beta1.RenovatorOperation)

	if err := r.Update(ctx, r.instance); err != nil {
		return &ctrl.Result{}, err
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateCronJob(job *batchv1.CronJob) error {
	job.Spec.Schedule = r.instance.Spec.Schedule
	job.Spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
	job.Spec.Suspend = r.instance.Spec.Suspend

	r.updateJobSpec(&job.Spec.JobTemplate.Spec)

	return nil
}

func (r *Reconciler) updateJobSpec(spec *batchv1.JobSpec) {
	spec.Template.Spec.ServiceAccountName = metadata.GenericMetadata(r.req).Name
	spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	renovateConfigCM := metadata.GenericName(r.req, renovateconfig.ConfigMapSuffix)

	spec.Template.Spec.Volumes = containers.VolumesTemplate(
		containers.WithEmptyDirVolume(renovate.VolumeRenovateTmp),
		containers.WithConfigMapVolume(renovate.VolumeRenovateConfig, renovateConfigCM),
	)

	spec.Template.Spec.InitContainers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate-init",
			r.renovate.Spec.Image,
			r.renovate.Spec.ImagePullPolicy,
			containers.WithContainerArgs([]string{
				"--write-discovered-repos",
				renovate.FileRenovateRepositories,
			}),
			containers.WithEnvVars(renovate.DefaultEnvVars(&r.renovate.Spec)),
			containers.WithEnvVars(
				[]corev1.EnvVar{
					{
						Name:  "RENOVATE_AUTODISCOVER",
						Value: "true",
					},
					{
						Name:  "RENOVATE_AUTODISCOVER_FILTER",
						Value: strings.Join(r.instance.Spec.Filter, ","),
					},
				},
			),
			containers.WithVolumeMounts(
				[]corev1.VolumeMount{
					{
						Name:      renovate.VolumeRenovateTmp,
						MountPath: renovate.DirRenovateTmp,
					},
					{
						Name:      renovate.VolumeRenovateConfig,
						MountPath: renovate.DirRenovateConfig,
					},
				},
			),
		),
	}

	spec.Template.Spec.Containers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate-discovery",
			r.instance.Spec.Image,
			r.instance.Spec.ImagePullPolicy,
			containers.WithContainerCommand([]string{"/discovery"}),
			containers.WithEnvVars(
				[]corev1.EnvVar{
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
			),
			containers.WithVolumeMounts(
				[]corev1.VolumeMount{
					{
						Name:      renovate.VolumeRenovateTmp,
						MountPath: renovate.DirRenovateTmp,
					},
				},
			),
		),
	}
}
