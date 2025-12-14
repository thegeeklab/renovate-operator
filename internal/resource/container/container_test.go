package containers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
)

func TestContainerTemplate(t *testing.T) {
	t.Parallel()

	baseEnvVars := []corev1.EnvVar{
		{Name: "TEST_VAR", Value: "test-value"},
	}

	// Test basic container creation
	container := containers.ContainerTemplate(
		"test-container",
		"nginx:latest",
		corev1.PullAlways,
		containers.WithEnvVars(baseEnvVars),
	)

	assert.Equal(t, "test-container", container.Name)
	assert.Equal(t, "nginx:latest", container.Image)
	assert.Equal(t, corev1.PullAlways, container.ImagePullPolicy)
	assert.Len(t, container.Env, 1)
	assert.Equal(t, "TEST_VAR", container.Env[0].Name)
	assert.Equal(t, "test-value", container.Env[0].Value)

	// Test with mutators
	container = containers.ContainerTemplate(
		"test-container",
		"nginx:latest",
		corev1.PullAlways,
		containers.WithContainerArgs([]string{"arg1", "arg2"}),
		containers.WithContainerCommand([]string{"cmd1", "cmd2"}),
		containers.WithEnvVars([]corev1.EnvVar{
			{Name: "ADDITIONAL_VAR", Value: "additional-value"},
		}),
	)

	assert.Equal(t, []string{"arg1", "arg2"}, container.Args)
	assert.Equal(t, []string{"cmd1", "cmd2"}, container.Command)
	assert.Len(t, container.Env, 1) // Only the additional env var should be present
}

func TestWithVolumeMounts(t *testing.T) {
	t.Parallel()

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "test-volume",
			MountPath: "/test/path",
		},
	}

	container := containers.ContainerTemplate(
		"test-container",
		"nginx:latest",
		corev1.PullAlways,
		containers.WithVolumeMounts(volumeMounts),
	)

	assert.Len(t, container.VolumeMounts, 1)
	assert.Equal(t, "test-volume", container.VolumeMounts[0].Name)
	assert.Equal(t, "/test/path", container.VolumeMounts[0].MountPath)
}

func TestWithResourceRequirements(t *testing.T) {
	t.Parallel()

	requirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}

	container := containers.ContainerTemplate(
		"test-container",
		"nginx:latest",
		corev1.PullAlways,
		containers.WithResourceRequirements(requirements),
	)

	assert.Equal(t, "1", container.Resources.Limits.Cpu().String())
	assert.Equal(t, "512Mi", container.Resources.Limits.Memory().String())
	assert.Equal(t, "500m", container.Resources.Requests.Cpu().String())
	assert.Equal(t, "256Mi", container.Resources.Requests.Memory().String())
}

func TestWithSecurityContext(t *testing.T) {
	t.Parallel()

	securityContext := &corev1.SecurityContext{
		RunAsNonRoot: ptr.To(true),
		RunAsUser:    ptr.To(int64(1000)),
	}

	container := containers.ContainerTemplate(
		"test-container",
		"nginx:latest",
		corev1.PullAlways,
		containers.WithSecurityContext(securityContext),
	)

	assert.NotNil(t, container.SecurityContext)
	assert.True(t, *container.SecurityContext.RunAsNonRoot)
	assert.Equal(t, int64(1000), *container.SecurityContext.RunAsUser)
}

func TestVolumesTemplate(t *testing.T) {
	t.Parallel()

	// Test basic volumes creation
	volumes := containers.VolumesTemplate()
	assert.Len(t, volumes, 0)

	// Test with mutators
	volumes = containers.VolumesTemplate(
		containers.WithConfigMapVolume("test-config", "test-configmap"),
		containers.WithEmptyDirVolume("test-empty-dir"),
		containers.WithSecretVolume("test-secret", "test-secret-name"),
		containers.WithPersistentVolumeClaim("test-pvc", "test-pvc-claim"),
	)

	assert.Len(t, volumes, 4)

	// Verify config volume
	configVolume := volumes[0]
	assert.Equal(t, "test-config", configVolume.Name)
	assert.NotNil(t, configVolume.ConfigMap)
	assert.Equal(t, "test-configmap", configVolume.ConfigMap.Name)

	// Verify empty dir volume
	emptyDirVolume := volumes[1]
	assert.Equal(t, "test-empty-dir", emptyDirVolume.Name)
	assert.NotNil(t, emptyDirVolume.EmptyDir)

	// Verify secret volume
	secretVolume := volumes[2]
	assert.Equal(t, "test-secret", secretVolume.Name)
	assert.NotNil(t, secretVolume.Secret)
	assert.Equal(t, "test-secret-name", secretVolume.Secret.SecretName)

	// Verify PVC volume
	pvcVolume := volumes[3]
	assert.Equal(t, "test-pvc", pvcVolume.Name)
	assert.NotNil(t, pvcVolume.PersistentVolumeClaim)
	assert.Equal(t, "test-pvc-claim", pvcVolume.PersistentVolumeClaim.ClaimName)
}

func TestWithConfigVolume(t *testing.T) {
	t.Parallel()

	volumes := containers.VolumesTemplate(
		containers.WithConfigMapVolume("test-volume", "test-configmap"),
	)

	assert.Len(t, volumes, 1)
	assert.Equal(t, "test-volume", volumes[0].Name)
	assert.NotNil(t, volumes[0].ConfigMap)
	assert.Equal(t, "test-configmap", volumes[0].ConfigMap.Name)
}
