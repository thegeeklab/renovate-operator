package renovate

import (
	"path/filepath"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

const (
	VolumeRenovateConfig = "renovate-config"
	VolumeRenovateTmp    = "renovate-tmp"

	DirRenovateConfig = "/etc/config/renovate"
	DirRenovateTmp    = "/tmp/renovate"

	FilenameRenovateConfig = "renovate.json"
	FilenameRepositories   = "repositories.json"
	FilenameIndex          = "index.json"

	EnvRenovateConfigRaw  = "RENOVATE_CONFIG_FILE_RAW"
	EnvRenovateConfig     = "RENOVATE_CONFIG_FILE"
	EnvRenovateIndex      = "RENOVATE_INDEX"
	EnvJobCompletionIndex = "JOB_COMPLETION_INDEX"

	// LabelRepoName is the label key used for the locking mechanism.
	// The controller uses this to find active jobs for a specific repository.
	LabelRepoName = "renovate.thegeeklab.de/repo"
)

var (
	FileRenovateConfig       = filepath.Join(DirRenovateConfig, FilenameRenovateConfig)
	FileRenovateTmp          = filepath.Join(DirRenovateTmp, FilenameRenovateConfig)
	FileRenovateRepositories = filepath.Join(DirRenovateTmp, FilenameRepositories)
	FileRenovateIndex        = filepath.Join(DirRenovateTmp, FilenameIndex)
)

func DefaultEnvVars(renovate *renovatev1beta1.RenovateConfigSpec) []corev1.EnvVar {
	containerVars := []corev1.EnvVar{
		{
			Name:  "LOG_LEVEL",
			Value: string(renovate.Logging.Level),
		},
		{
			Name:  EnvRenovateConfig,
			Value: FileRenovateConfig,
		},
		{
			Name:      "RENOVATE_TOKEN",
			ValueFrom: &renovate.Platform.Token,
		},
	}
	if renovate.GithubToken != nil {
		containerVars = append(containerVars, corev1.EnvVar{
			Name:      "GITHUB_COM_TOKEN",
			ValueFrom: renovate.GithubToken,
		})
	}

	return containerVars
}
