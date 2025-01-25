package renovate

import (
	"path/filepath"

	"github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

const (
	VolumeConfig    = "config"
	VolumeRawConfig = "raw-config"
	VolumeWorkDir   = "workdir"

	DirRenovateBase   = "/tmp/renovate"
	DirRenovateConfig = "/etc/config/renovate"
	DirRawConfig      = "/tmp/rawConfigs"
)

var (
	FileRenovateConfig       = filepath.Join(DirRenovateConfig, "instance.json")
	FileRenovateConfigOutput = filepath.Join(DirRenovateBase, "repositories.json")
)

func Container(renovate v1beta1.Renovator, additionalEnVars []corev1.EnvVar, additionalArgs []string) corev1.Container {
	return corev1.Container{
		Name:  "renovate",
		Image: ContainerImage(renovate),
		Args:  additionalArgs,
		Env:   append(EnvVars(renovate), additionalEnVars...),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      VolumeWorkDir,
				MountPath: DirRenovateBase,
			},
			{
				Name:      VolumeConfig,
				ReadOnly:  true,
				MountPath: DirRenovateConfig,
			},
		},
	}
}

func EnvVars(instance v1beta1.Renovator) []corev1.EnvVar {
	containerVars := []corev1.EnvVar{
		{
			Name:  "LOG_LEVEL",
			Value: string(instance.Spec.Logging.Level),
		},
		{
			Name:  "RENOVATE_BASE_DIR",
			Value: DirRenovateBase,
		},
		{
			Name:  "RENOVATE_CONFIG_FILE",
			Value: FileRenovateConfig,
		},
		{
			Name:      "RENOVATE_TOKEN",
			ValueFrom: &instance.Spec.RenovateConfig.Platform.Token,
		},
	}
	if instance.Spec.RenovateConfig.GithubTokenSelector.Size() != 0 {
		containerVars = append(containerVars, corev1.EnvVar{
			Name:      "GITHUB_COM_TOKEN",
			ValueFrom: &instance.Spec.RenovateConfig.GithubTokenSelector,
		})
	}

	return containerVars
}

func ContainerImage(instance v1beta1.Renovator) string {
	return "renovate/renovate:" + instance.Spec.RenovateConfig.RenovateVersion
}

func StandardVolumes(volumeConfigVolumeSource corev1.VolumeSource) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: VolumeWorkDir,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name:         VolumeConfig,
			VolumeSource: volumeConfigVolumeSource,
		},
	}
}
