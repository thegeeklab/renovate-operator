package containers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// ContainerMutator defines a function type for mutating container configurations.
type ContainerMutator func(*corev1.Container)

// VolumeMutator defines a function type for mutating volume configurations.
type VolumeMutator func(*[]corev1.Volume)

// ContainerTemplate creates a default container with optional mutators.
func ContainerTemplate(
	name string,
	image string,
	imagePullPolicy corev1.PullPolicy,
	mutators ...ContainerMutator,
) corev1.Container {
	container := corev1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
	}

	// Apply all mutators to the container
	for _, mutator := range mutators {
		mutator(&container)
	}

	return container
}

// WithContainerArgs sets the container arguments.
func WithContainerArgs(args []string) ContainerMutator {
	return func(c *corev1.Container) {
		c.Args = args
	}
}

// WithContainerCommand sets the container command.
func WithContainerCommand(cmd []string) ContainerMutator {
	return func(c *corev1.Container) {
		c.Command = cmd
	}
}

// WithEnvVars adds additional environment variables.
func WithEnvVars(envVars []corev1.EnvVar) ContainerMutator {
	return func(c *corev1.Container) {
		c.Env = append(c.Env, envVars...)
	}
}

// WithVolumeMounts adds additional volume mounts.
func WithVolumeMounts(mounts []corev1.VolumeMount) ContainerMutator {
	return func(c *corev1.Container) {
		c.VolumeMounts = append(c.VolumeMounts, mounts...)
	}
}

// WithImagePullPolicy sets the image pull policy.
func WithImagePullPolicy(policy corev1.PullPolicy) ContainerMutator {
	return func(c *corev1.Container) {
		c.ImagePullPolicy = policy
	}
}

// WithResourceRequirements sets the resource requirements.
func WithResourceRequirements(requirements corev1.ResourceRequirements) ContainerMutator {
	return func(c *corev1.Container) {
		c.Resources = requirements
	}
}

// WithSecurityContext sets the security context.
func WithSecurityContext(context *corev1.SecurityContext) ContainerMutator {
	return func(c *corev1.Container) {
		c.SecurityContext = context
	}
}

// VolumesTemplate creates default volumes with optional mutators.
func VolumesTemplate(mutators ...VolumeMutator) []corev1.Volume {
	volumes := []corev1.Volume{}

	// Apply all mutators to the volumes
	for _, mutator := range mutators {
		mutator(&volumes)
	}

	return volumes
}

// WithEmptyDirVolume adds an empty directory volume.
func WithEmptyDirVolume(name string) VolumeMutator {
	return func(volumes *[]corev1.Volume) {
		*volumes = append(*volumes, corev1.Volume{
			Name:         name,
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		})
	}
}

// WithConfigMapVolume adds a ConfigMap volume.
func WithConfigMapVolume(name, configMapName string) VolumeMutator {
	return func(volumes *[]corev1.Volume) {
		*volumes = append(*volumes, corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: ptr.To(corev1.ConfigMapVolumeSourceDefaultMode),
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		})
	}
}

// WithSecretVolume adds a secret volume.
func WithSecretVolume(name, secretName string) VolumeMutator {
	return func(volumes *[]corev1.Volume) {
		*volumes = append(*volumes, corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: ptr.To(corev1.SecretVolumeSourceDefaultMode),
					SecretName:  secretName,
				},
			},
		})
	}
}

// WithPersistentVolumeClaim adds a persistent volume claim.
func WithPersistentVolumeClaim(name, claimName string) VolumeMutator {
	return func(volumes *[]corev1.Volume) {
		*volumes = append(*volumes, corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: claimName,
				},
			},
		})
	}
}
