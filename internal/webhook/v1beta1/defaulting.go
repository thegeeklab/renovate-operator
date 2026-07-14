package v1beta1

import (
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// defaultScratchVolume sets default values for scratch volume configuration.
func defaultScratchVolume(scratch *renovatev1beta1.ScratchVolumeSpec) {
	if scratch == nil || scratch.Ephemeral != nil {
		return
	}

	if scratch.Path == "" {
		scratch.Path = renovatev1beta1.DefaultScratchVolumePath
	}

	if scratch.Medium == corev1.StorageMediumMemory && scratch.SizeLimit == nil {
		defaultLimit := resource.MustParse("1Gi")
		scratch.SizeLimit = &defaultLimit
	}
}
