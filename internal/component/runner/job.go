package runner

import (
	"context"

	renovateconfig "github.com/thegeeklab/renovate-operator/internal/component/renovate-config"
	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	cronjob "github.com/thegeeklab/renovate-operator/internal/resource/cronjob"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *Reconciler) reconcileCronJob(ctx context.Context) (*ctrl.Result, error) {
	// Check if immediate renovate is requested via annotation
	if renovator.HasRenovatorOperationRenovate(r.instance.Annotations) {
		return r.handleImmediateRenovate(ctx)
	}

	job := &batchv1.CronJob{ObjectMeta: RunnerMetadata(r.req)}

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

func (r *Reconciler) handleImmediateRenovate(ctx context.Context) (*ctrl.Result, error) {
	// Check for active renovate jobs with our specific labels
	active, err := cronjob.CheckActiveJobs(ctx, r.Client, r.instance.Namespace, RunnerName(r.req))
	if err != nil {
		return &ctrl.Result{}, err
	}

	if active {
		return &ctrl.Result{RequeueAfter: cronjob.RequeueDelay}, nil
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: RunnerName(r.req) + "-",
			Namespace:    r.instance.Namespace,
		},
		Spec: batchv1.JobSpec{},
	}

	r.updateJobSpec(&job.Spec)

	_, err = k8s.CreateOrPatch(ctx, r.Client, job, r.instance, nil)
	if err != nil {
		return &ctrl.Result{}, err
	}

	// Remove renovate annotation
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
	renovateConfigCM := metadata.GenericName(r.req, renovateconfig.ConfigMapSuffix)
	renovateBatchesCM := metadata.GenericName(r.req, ConfigMapSuffix)

	spec.CompletionMode = ptr.To(batchv1.IndexedCompletion)
	spec.Completions = ptr.To(r.batchesCount)
	spec.Parallelism = ptr.To(r.instance.Spec.Instances)
	spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	spec.Template.Spec.Volumes = containers.VolumesTemplate(
		containers.WithEmptyDirVolume(renovate.VolumeRenovateConfig),
		containers.WithConfigMapVolume(renovateConfigCM, renovateConfigCM),
		containers.WithConfigMapVolume(renovateBatchesCM, renovateBatchesCM),
	)

	spec.Template.Spec.InitContainers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate-dispatcher",
			r.instance.Spec.Image,
			r.instance.Spec.ImagePullPolicy,
			containers.WithEnvVars([]corev1.EnvVar{
				{
					Name:  renovate.EnvRenovateConfigRaw,
					Value: renovate.FileRenovateTmp,
				},
				{
					Name:  renovate.EnvRenovateConfig,
					Value: renovate.FileRenovateConfig,
				},
				{
					Name:  renovate.EnvRenovateBatches,
					Value: renovate.FileRenovateBatches,
				},
			}),
			containers.WithContainerCommand([]string{"/dispatcher"}),
			containers.WithVolumeMounts([]corev1.VolumeMount{
				{
					Name:      renovate.VolumeRenovateConfig,
					MountPath: renovate.DirRenovateConfig,
				},
				{
					Name:      renovateConfigCM,
					ReadOnly:  true,
					MountPath: renovate.FileRenovateTmp,
					SubPath:   renovate.FilenameRenovateConfig,
				},
				{
					Name:      renovateBatchesCM,
					ReadOnly:  true,
					MountPath: renovate.FileRenovateBatches,
					SubPath:   renovate.FilenameBatches,
				},
			}),
		),
	}
	spec.Template.Spec.Containers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate",
			r.renovate.Spec.Image,
			r.renovate.Spec.ImagePullPolicy,
			containers.WithEnvVars(renovate.DefaultEnvVars(&r.renovate.Spec)),
			containers.WithVolumeMounts(
				[]corev1.VolumeMount{
					{
						Name:      renovate.VolumeRenovateConfig,
						MountPath: renovate.DirRenovateConfig,
					},
				},
			),
		),
	}
}
