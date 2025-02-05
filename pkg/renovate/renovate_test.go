package renovate

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("DefaultContainer", func() {
	var instance *renovatev1beta1.Renovator

	BeforeEach(func() {
		instance = &renovatev1beta1.Renovator{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-renovator",
			},
			Spec: renovatev1beta1.RenovatorSpec{
				Renovate: renovatev1beta1.RenovateSpec{
					Image: "renovate/renovate:latest",
					Platform: renovatev1beta1.PlatformSpec{
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "test-secret",
								},
								Key: "token",
							},
						},
					},
				},
				ImagePullPolicy: corev1.PullIfNotPresent,
				Logging: renovatev1beta1.LoggingSpec{
					Level: "info",
				},
			},
		}
	})

	It("should create container with default configuration", func() {
		container := DefaultContainer(instance, nil, nil)

		Expect(container.Name).To(Equal("renovate"))
		Expect(container.Image).To(Equal("renovate/renovate:latest"))
		Expect(container.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		Expect(container.Args).To(BeEmpty())
		Expect(container.VolumeMounts).To(HaveLen(2))
	})

	It("should append additional environment variables", func() {
		additionalEnvVars := []corev1.EnvVar{
			{Name: "EXTRA_VAR", Value: "extra-value"},
		}
		container := DefaultContainer(instance, additionalEnvVars, nil)

		Expect(container.Env).To(ContainElement(additionalEnvVars[0]))
	})

	It("should append additional arguments", func() {
		additionalArgs := []string{"--dry-run", "--debug"}
		container := DefaultContainer(instance, nil, additionalArgs)

		Expect(container.Args).To(Equal(additionalArgs))
	})
})

var _ = Describe("DefaultEnvVars", func() {
	var instance *renovatev1beta1.Renovator

	BeforeEach(func() {
		instance = &renovatev1beta1.Renovator{
			Spec: renovatev1beta1.RenovatorSpec{
				Logging: renovatev1beta1.LoggingSpec{
					Level: "debug",
				},
				Renovate: renovatev1beta1.RenovateSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "platform-token",
								},
								Key: "token",
							},
						},
					},
				},
			},
		}
	})

	It("should include required environment variables", func() {
		envVars := DefaultEnvVars(instance)

		Expect(envVars).To(ContainElements(
			corev1.EnvVar{Name: "LOG_LEVEL", Value: "debug"},
			corev1.EnvVar{Name: "RENOVATE_BASE_DIR", Value: DirRenovateTmp},
			corev1.EnvVar{Name: "RENOVATE_CONFIG_FILE", Value: FileRenovateConfig},
		))
	})

	It("should include GitHub token when selector is provided", func() {
		instance.Spec.Renovate.GithubTokenSelector = &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "github-token",
				},
				Key: "token",
			},
		}

		envVars := DefaultEnvVars(instance)
		Expect(envVars).To(ContainElement(
			corev1.EnvVar{
				Name:      "GITHUB_COM_TOKEN",
				ValueFrom: instance.Spec.Renovate.GithubTokenSelector,
			},
		))
	})
})

var _ = Describe("DefaultVolume", func() {
	It("should create volumes with provided config source", func() {
		configSource := corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "test-config",
				},
			},
		}

		volumes := DefaultVolume(configSource)

		Expect(volumes).To(HaveLen(1))
		Expect(volumes[0].Name).To(Equal(VolumeRenovateConfig))
		Expect(volumes[0].VolumeSource).To(Equal(configSource))
	})
})
