package renovate

import (
	"github.com/thegeeklab/renovate-operator/api/v1beta1"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func DefaultJobSpec(spec *batchv1.JobSpec, bc int32, batchesCM, renovateCM string, runner *v1beta1.Runner, renovate *v1beta1.RenovateConfig) {
	spec.CompletionMode = ptr.To(batchv1.IndexedCompletion)
	spec.Completions = ptr.To(bc)
	spec.Parallelism = ptr.To(runner.Spec.Instances)
	spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	spec.Template.Spec.Volumes = containers.VolumesTemplate(
		containers.WithEmptyDirVolume(VolumeRenovateConfig),
		containers.WithConfigMapVolume(renovateCM, renovateCM),
		containers.WithConfigMapVolume(batchesCM, batchesCM),
	)

	spec.Template.Spec.InitContainers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate-dispatcher",
			runner.Spec.Image,
			runner.Spec.ImagePullPolicy,
			containers.WithEnvVars([]corev1.EnvVar{
				{
					Name:  EnvRenovateConfigRaw,
					Value: FileRenovateTmp,
				},
				{
					Name:  EnvRenovateConfig,
					Value: FileRenovateConfig,
				},
				{
					Name:  EnvRenovateBatches,
					Value: FileRenovateBatches,
				},
			}),
			containers.WithContainerCommand([]string{"/dispatcher"}),
			containers.WithVolumeMounts([]corev1.VolumeMount{
				{
					Name:      VolumeRenovateConfig,
					MountPath: DirRenovateConfig,
				},
				{
					Name:      renovateCM,
					ReadOnly:  true,
					MountPath: FileRenovateTmp,
					SubPath:   FilenameRenovateConfig,
				},
				{
					Name:      batchesCM,
					ReadOnly:  true,
					MountPath: FileRenovateBatches,
					SubPath:   FilenameBatches,
				},
			}),
		),
	}

	spec.Template.Spec.Containers = []corev1.Container{
		containers.ContainerTemplate(
			"renovate",
			renovate.Spec.Image,
			renovate.Spec.ImagePullPolicy,
			containers.WithEnvVars(DefaultEnvVars(&renovate.Spec)),
			containers.WithVolumeMounts(
				[]corev1.VolumeMount{
					{
						Name:      VolumeRenovateConfig,
						MountPath: DirRenovateConfig,
					},
				},
			),
		),
	}
}
