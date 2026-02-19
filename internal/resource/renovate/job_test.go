package renovate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
)

var _ = Describe("JobSpec", func() {
	var (
		runner     *renovatev1beta1.Runner
		renovateCR *renovatev1beta1.RenovateConfig
		renovateCM string
	)

	BeforeEach(func() {
		runner = &renovatev1beta1.Runner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runner",
				Namespace: "test-namespace",
			},
			Spec: renovatev1beta1.RunnerSpec{
				Instances: 3,
				ImageSpec: renovatev1beta1.ImageSpec{
					Image: "renovate/renovate:latest",
				},
			},
		}

		renovateCR = &renovatev1beta1.RenovateConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-renovate",
				Namespace: "test-namespace",
			},
			Spec: renovatev1beta1.RenovateConfigSpec{
				ImageSpec: renovatev1beta1.ImageSpec{
					Image: "renovate/renovate:latest",
				},
				Platform: renovatev1beta1.PlatformSpec{
					Token: corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "token",
						},
					},
				},
				Logging: &renovatev1beta1.LoggingSpec{
					Level: renovatev1beta1.LogLevel_DEBUG,
				},
			},
		}

		renovateCM = "test-renovate-config"
	})

	Describe("DefaultJobSpec", func() {
		It("should create a basic job spec with default configuration", func() {
			// Create a new JobSpec
			jobSpec := &batchv1.JobSpec{}
			renovate.DefaultJobSpec(jobSpec, renovateCR, renovateCM)

			// Verify basic job spec configuration
			Expect(jobSpec.CompletionMode).To(BeNil())
			Expect(jobSpec.Completions).To(BeNil())
			Expect(jobSpec.Parallelism).To(Equal(ptr.To(int32(3))))
			Expect(jobSpec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))

			// Verify volumes
			Expect(jobSpec.Template.Spec.Volumes).To(HaveLen(2))
			Expect(jobSpec.Template.Spec.Volumes[0].Name).To(Equal("renovate-config"))
			Expect(jobSpec.Template.Spec.Volumes[0].EmptyDir).NotTo(BeNil())
			Expect(jobSpec.Template.Spec.Volumes[1].Name).To(Equal(renovateCM))
			Expect(jobSpec.Template.Spec.Volumes[1].ConfigMap).NotTo(BeNil())
			Expect(jobSpec.Template.Spec.Volumes[1].ConfigMap.Name).To(Equal(renovateCM))

			// Verify main container
			Expect(jobSpec.Template.Spec.Containers).To(HaveLen(1))
			mainContainer := jobSpec.Template.Spec.Containers[0]
			Expect(mainContainer.Name).To(Equal("renovate"))
			Expect(mainContainer.Image).To(Equal("renovate/renovate:latest"))
			Expect(mainContainer.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))

			// Verify volume mounts
			Expect(mainContainer.VolumeMounts).To(HaveLen(1))
			Expect(mainContainer.VolumeMounts[0].Name).To(Equal("renovate-config"))
			Expect(mainContainer.VolumeMounts[0].MountPath).To(Equal("/etc/config/renovate"))

			// Verify environment variables
			Expect(mainContainer.Env).To(HaveLen(3)) // LOG_LEVEL, RENOVATE_CONFIG_FILE, RENOVATE_TOKEN

			envMap := make(map[string]string)
			for _, env := range mainContainer.Env {
				envMap[env.Name] = env.Value
			}

			Expect(envMap["LOG_LEVEL"]).To(Equal("debug"))
			Expect(envMap["RENOVATE_CONFIG_FILE"]).To(Equal("/etc/config/renovate/renovate.json"))
			Expect(envMap["RENOVATE_TOKEN"]).To(Equal(""))
		})

		It("should apply job options to modify the configuration", func() {
			// Create a new JobSpec
			jobSpec := &batchv1.JobSpec{}

			// Apply custom options
			renovate.DefaultJobSpec(
				jobSpec,
				renovateCR,
				renovateCM,
				renovate.WithSingleRepoMode("test-org/test-repo"),
			)

			// Verify single repo mode configuration
			Expect(*jobSpec.CompletionMode).To(Equal(batchv1.NonIndexedCompletion))
			Expect(*jobSpec.Completions).To(Equal(int32(1)))
			Expect(*jobSpec.Parallelism).To(Equal(int32(1)))

			// Verify repository override environment variable
			mainContainer := jobSpec.Template.Spec.Containers[0]

			envMap := make(map[string]string)
			for _, env := range mainContainer.Env {
				envMap[env.Name] = env.Value
			}

			Expect(envMap["RENOVATE_REPOSITORIES"]).To(Equal("test-org/test-repo"))
		})
	})

	Describe("WithIndexMode", func() {
		It("should configure job for index mode with dispatcher", func() {
			// Create a new JobSpec
			jobSpec := &batchv1.JobSpec{}

			// Apply index mode
			renovate.DefaultJobSpec(
				jobSpec,
				renovateCR,
				renovateCM,
				renovate.WithIndexMode(runner, "test-index", 5),
			)

			// Verify index mode configuration
			Expect(*jobSpec.CompletionMode).To(Equal(batchv1.IndexedCompletion))
			Expect(*jobSpec.Completions).To(Equal(int32(5)))
			Expect(*jobSpec.Parallelism).To(Equal(int32(3)))

			// Verify additional volumes
			Expect(jobSpec.Template.Spec.Volumes).To(HaveLen(3))
			Expect(jobSpec.Template.Spec.Volumes[2].Name).To(Equal("test-index"))
			Expect(jobSpec.Template.Spec.Volumes[2].ConfigMap).NotTo(BeNil())
			Expect(jobSpec.Template.Spec.Volumes[2].ConfigMap.Name).To(Equal("test-index"))

			// Verify init containers
			Expect(jobSpec.Template.Spec.InitContainers).To(HaveLen(1))
			dispatcher := jobSpec.Template.Spec.InitContainers[0]
			Expect(dispatcher.Name).To(Equal("renovate-dispatcher"))
			Expect(dispatcher.Image).To(Equal("renovate/renovate:latest"))
			Expect(dispatcher.Command).To(Equal([]string{"/dispatcher"}))

			// Verify dispatcher volume mounts
			Expect(dispatcher.VolumeMounts).To(HaveLen(3))

			dispatcherMountMap := make(map[string]string)
			for _, vm := range dispatcher.VolumeMounts {
				dispatcherMountMap[vm.Name] = vm.MountPath
			}

			Expect(dispatcherMountMap["renovate-config"]).To(Equal("/etc/config/renovate"))
			Expect(dispatcherMountMap[renovateCM]).To(Equal("/tmp/renovate/renovate.json"))
			Expect(dispatcherMountMap["test-index"]).To(Equal("/tmp/renovate/index.json"))

			// Verify dispatcher environment variables
			dispatcherEnvMap := make(map[string]string)
			for _, env := range dispatcher.Env {
				dispatcherEnvMap[env.Name] = env.Value
			}

			Expect(dispatcherEnvMap["RENOVATE_CONFIG_FILE_RAW"]).To(Equal("/tmp/renovate/renovate.json"))
			Expect(dispatcherEnvMap["RENOVATE_CONFIG_FILE"]).To(Equal("/etc/config/renovate/renovate.json"))
			Expect(dispatcherEnvMap["RENOVATE_INDEX"]).To(Equal("/tmp/renovate/index.json"))
		})
	})

	Describe("WithSingleRepoMode", func() {
		It("should configure job for single repository run", func() {
			// Create a new JobSpec
			jobSpec := &batchv1.JobSpec{}

			// Apply single repo mode
			renovate.DefaultJobSpec(
				jobSpec,
				renovateCR,
				renovateCM,
				renovate.WithSingleRepoMode("test-org/test-repo"),
			)

			// Verify single repo mode configuration
			Expect(*jobSpec.CompletionMode).To(Equal(batchv1.NonIndexedCompletion))
			Expect(*jobSpec.Completions).To(Equal(int32(1)))
			Expect(*jobSpec.Parallelism).To(Equal(int32(1)))

			// Verify repository override environment variable
			mainContainer := jobSpec.Template.Spec.Containers[0]

			envMap := make(map[string]string)
			for _, env := range mainContainer.Env {
				envMap[env.Name] = env.Value
			}

			Expect(envMap["RENOVATE_REPOSITORIES"]).To(Equal("test-org/test-repo"))

			// Verify no init containers for single repo mode
			Expect(jobSpec.Template.Spec.InitContainers).To(BeEmpty())
		})
	})

	Describe("Multiple JobOptions", func() {
		It("should apply multiple job options correctly", func() {
			// Create a new JobSpec
			jobSpec := &batchv1.JobSpec{}

			renovate.DefaultJobSpec(
				jobSpec,
				renovateCR,
				renovateCM,
				renovate.WithSingleRepoMode("test-org/test-repo"),
			)

			// The last applied option should take precedence for conflicting fields
			Expect(*jobSpec.CompletionMode).To(Equal(batchv1.NonIndexedCompletion))
			Expect(*jobSpec.Completions).To(Equal(int32(1)))
			Expect(*jobSpec.Parallelism).To(Equal(int32(1)))
		})
	})
})
