package renovate

import (
	"path/filepath"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

const (
	VolumeRenovateConfig = "renovate-config"
	VolumeRenovateTmp    = "renovate-tmp"
	VolumeRenovateBase   = "renovate-base"

	DirRenovateConfig = "/etc/config/renovate"
	DirRenovateTmp    = "/tmp/renovate"
)

var (
	FileRenovateConfig       = filepath.Join(DirRenovateConfig, "renovate.json")
	FileRenovateTmp          = filepath.Join(DirRenovateTmp, "renovate.json")
	FileRenovateRepositories = filepath.Join(DirRenovateTmp, "repositories.json")
	FileRenovateBatches      = filepath.Join(DirRenovateTmp, "batches.json")
)

func DefaultContainer(
	instance *renovatev1beta1.Renovator,
	additionalEnVars []corev1.EnvVar,
	additionalArgs []string,
) corev1.Container {
	return corev1.Container{
		Name:            "renovate",
		Image:           instance.Spec.Renovate.Image,
		ImagePullPolicy: instance.Spec.ImagePullPolicy,
		Args:            additionalArgs,
		Env:             append(DefaultEnvVars(instance), additionalEnVars...),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      VolumeRenovateBase,
				MountPath: DirRenovateTmp,
			},
			{
				Name:      VolumeRenovateConfig,
				ReadOnly:  true,
				MountPath: DirRenovateConfig,
			},
		},
	}
}

func DefaultEnvVars(instance *renovatev1beta1.Renovator) []corev1.EnvVar {
	containerVars := []corev1.EnvVar{
		{
			Name:  "LOG_LEVEL",
			Value: string(instance.Spec.Logging.Level),
		},
		{
			Name:  "RENOVATE_BASE_DIR",
			Value: DirRenovateTmp,
		},
		{
			Name:  "RENOVATE_CONFIG_FILE",
			Value: FileRenovateConfig,
		},
		{
			Name:      "RENOVATE_TOKEN",
			ValueFrom: &instance.Spec.Renovate.Platform.Token,
		},
	}
	if instance.Spec.Renovate.GithubTokenSelector != nil {
		containerVars = append(containerVars, corev1.EnvVar{
			Name:      "GITHUB_COM_TOKEN",
			ValueFrom: instance.Spec.Renovate.GithubTokenSelector,
		})
	}

	return containerVars
}

func DefaultVolume(volumeConfigVolumeSource corev1.VolumeSource) []corev1.Volume {
	return []corev1.Volume{
		{
			Name:         VolumeRenovateConfig,
			VolumeSource: volumeConfigVolumeSource,
		},
	}
}
