package discovery

import (
	"context"
	"strings"

	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	cronjob "github.com/thegeeklab/renovate-operator/internal/resource/cronjob"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/discovery"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) reconcileCronJob(ctx context.Context) (*ctrl.Result, error) {
	// Check if immediate discovery is requested via annotation
	if renovator.HasRenovatorOperationDiscover(r.instance.Annotations) {
		return r.handleImmediateDiscovery(ctx)
	}

	job := &batchv1.CronJob{ObjectMeta: DiscoveryMetadata(r.req)}

	op, err := k8s.CreateOrUpdate(ctx, r.Client, job, r.instance, func() error {
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
	log := logf.FromContext(ctx)

	// Check for active discovery jobs with our specific labels
	active, err := cronjob.CheckActiveJobs(ctx, r.Client, r.instance.Namespace, DiscoveryName(r.req))
	if err != nil {
		return &ctrl.Result{}, err
	}

	if active {
		log.V(1).Info("Active discovery jobs found, requeuing", "delay", cronjob.RequeueDelay)

		return &ctrl.Result{RequeueAfter: cronjob.RequeueDelay}, nil
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: DiscoveryName(r.req) + "-",
			Namespace:    r.instance.Namespace,
		},
		Spec: batchv1.JobSpec{},
	}

	r.updateJobSpec(&job.Spec)

	if _, err := k8s.CreateOrUpdate(ctx, r.Client, job, r.instance, nil); err != nil {
		return &ctrl.Result{}, err
	}

	// Remove discovery annotation
	r.instance.Annotations = renovator.RemoveRenovatorOperation(r.instance.Annotations)
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

	renovateConfigCM := metadata.GenericName(r.req, renovator.ConfigMapSuffix)

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
						Name:  discovery.EnvDiscoveryInstanceName,
						Value: r.instance.Name,
					},
					{
						Name:  discovery.EnvDiscoveryInstanceNamespace,
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
