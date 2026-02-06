package containers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
)

var _ = Describe("Container Template", func() {
	Describe("ContainerTemplate", func() {
		It("should create basic container with environment variables", func() {
			baseEnvVars := []corev1.EnvVar{
				{Name: "TEST_VAR", Value: "test-value"},
			}

			container := containers.ContainerTemplate(
				"test-container",
				"nginx:latest",
				corev1.PullAlways,
				containers.WithEnvVars(baseEnvVars),
			)

			Expect(container.Name).To(Equal("test-container"))
			Expect(container.Image).To(Equal("nginx:latest"))
			Expect(container.ImagePullPolicy).To(Equal(corev1.PullAlways))
			Expect(container.Env).To(HaveLen(1))
			Expect(container.Env[0].Name).To(Equal("TEST_VAR"))
			Expect(container.Env[0].Value).To(Equal("test-value"))
		})

		It("should create container with multiple mutators", func() {
			container := containers.ContainerTemplate(
				"test-container",
				"nginx:latest",
				corev1.PullAlways,
				containers.WithContainerArgs([]string{"arg1", "arg2"}),
				containers.WithContainerCommand([]string{"cmd1", "cmd2"}),
				containers.WithEnvVars([]corev1.EnvVar{
					{Name: "ADDITIONAL_VAR", Value: "additional-value"},
				}),
			)

			Expect(container.Args).To(Equal([]string{"arg1", "arg2"}))
			Expect(container.Command).To(Equal([]string{"cmd1", "cmd2"}))
			Expect(container.Env).To(HaveLen(1))
		})
	})

	Describe("WithVolumeMounts", func() {
		It("should add volume mounts to container", func() {
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

			Expect(container.VolumeMounts).To(HaveLen(1))
			Expect(container.VolumeMounts[0].Name).To(Equal("test-volume"))
			Expect(container.VolumeMounts[0].MountPath).To(Equal("/test/path"))
		})
	})

	Describe("WithResourceRequirements", func() {
		It("should set resource requirements", func() {
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

			Expect(container.Resources.Limits.Cpu().String()).To(Equal("1"))
			Expect(container.Resources.Limits.Memory().String()).To(Equal("512Mi"))
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("500m"))
			Expect(container.Resources.Requests.Memory().String()).To(Equal("256Mi"))
		})
	})

	Describe("WithSecurityContext", func() {
		It("should set security context", func() {
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

			Expect(container.SecurityContext).NotTo(BeNil())
			Expect(*container.SecurityContext.RunAsNonRoot).To(BeTrue())
			Expect(*container.SecurityContext.RunAsUser).To(Equal(int64(1000)))
		})
	})

	Describe("VolumesTemplate", func() {
		It("should create empty volumes template", func() {
			volumes := containers.VolumesTemplate()
			Expect(volumes).To(BeEmpty())
		})

		It("should create volumes with multiple mutators", func() {
			volumes := containers.VolumesTemplate(
				containers.WithConfigMapVolume("test-config", "test-configmap"),
				containers.WithEmptyDirVolume("test-empty-dir"),
				containers.WithSecretVolume("test-secret", "test-secret-name"),
				containers.WithPersistentVolumeClaim("test-pvc", "test-pvc-claim"),
			)

			Expect(volumes).To(HaveLen(4))

			// Verify config volume
			configVolume := volumes[0]
			Expect(configVolume.Name).To(Equal("test-config"))
			Expect(configVolume.ConfigMap).NotTo(BeNil())
			Expect(configVolume.ConfigMap.Name).To(Equal("test-configmap"))

			// Verify empty dir volume
			emptyDirVolume := volumes[1]
			Expect(emptyDirVolume.Name).To(Equal("test-empty-dir"))
			Expect(emptyDirVolume.EmptyDir).NotTo(BeNil())

			// Verify secret volume
			secretVolume := volumes[2]
			Expect(secretVolume.Name).To(Equal("test-secret"))
			Expect(secretVolume.Secret).NotTo(BeNil())
			Expect(secretVolume.Secret.SecretName).To(Equal("test-secret-name"))

			// Verify PVC volume
			pvcVolume := volumes[3]
			Expect(pvcVolume.Name).To(Equal("test-pvc"))
			Expect(pvcVolume.PersistentVolumeClaim).NotTo(BeNil())
			Expect(pvcVolume.PersistentVolumeClaim.ClaimName).To(Equal("test-pvc-claim"))
		})
	})

	Describe("WithConfigVolume", func() {
		It("should create configmap volume", func() {
			volumes := containers.VolumesTemplate(
				containers.WithConfigMapVolume("test-volume", "test-configmap"),
			)

			Expect(volumes).To(HaveLen(1))
			Expect(volumes[0].Name).To(Equal("test-volume"))
			Expect(volumes[0].ConfigMap).NotTo(BeNil())
			Expect(volumes[0].ConfigMap.Name).To(Equal("test-configmap"))
		})
	})
})
