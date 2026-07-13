package renovate_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newJob(name, status string) *batchv1.Job {
	j := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "test-ns",
			Labels:            map[string]string{"app": "renovate-test"},
			CreationTimestamp: metav1.Now(),
		},
	}

	switch status {
	case "active":
		j.Status.Active = 1
	case "succeeded":
		j.Status.Succeeded = 1
	case "failed":
		j.Status.Failed = 1
	}

	return j
}

var _ = Describe("Renovate Job Library", func() {
	var (
		renovateCR *renovatev1beta1.RenovateConfig
		renovateCM string
	)

	BeforeEach(func() {
		renovateCR = &renovatev1beta1.RenovateConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-renovate",
				Namespace: "test-namespace",
			},
			Spec: renovatev1beta1.RenovateConfigSpec{
				ImageSpec: renovatev1beta1.ImageSpec{Image: "renovate/renovate:latest"},
				Platform: renovatev1beta1.PlatformSpec{
					Token: corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "token"}},
				},
				Logging: &renovatev1beta1.LoggingSpec{
					Level: renovatev1beta1.LogLevel_DEBUG,
				},
			},
		}
		renovateCM = "test-renovate-config"
	})

	Describe("DefaultJobSpec", func() {
		It("should create a valid default job spec", func() {
			jobSpec := &batchv1.JobSpec{}
			renovate.DefaultJobSpec(jobSpec, renovateCR, renovateCM)

			Expect(jobSpec.CompletionMode).To(Equal(new(batchv1.NonIndexedCompletion)))
			Expect(jobSpec.Parallelism).To(Equal(new(int32(1))))
			Expect(jobSpec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))
			Expect(jobSpec.Template.Spec.Volumes).To(HaveLen(2))
			Expect(jobSpec.Template.Spec.Containers).To(HaveLen(1))
			Expect(jobSpec.Template.Spec.ImagePullSecrets).To(BeEmpty())
		})

		It("should create scratch volume at /tmp by default", func() {
			jobSpec := &batchv1.JobSpec{}
			renovate.DefaultJobSpec(jobSpec, renovateCR, renovateCM)

			var scratchVol *corev1.Volume

			for i := range jobSpec.Template.Spec.Volumes {
				if jobSpec.Template.Spec.Volumes[i].Name == renovate.VolumeRenovateTmp {
					scratchVol = &jobSpec.Template.Spec.Volumes[i]

					break
				}
			}

			Expect(scratchVol).NotTo(BeNil())
			Expect(scratchVol.EmptyDir).NotTo(BeNil())

			env := jobSpec.Template.Spec.Containers[0].Env
			Expect(env).To(ContainElement(HaveField("Name", "RENOVATE_BASE_DIR")))
			Expect(env).To(ContainElement(HaveField("Value", "/tmp")))

			mounts := jobSpec.Template.Spec.Containers[0].VolumeMounts
			Expect(mounts).To(ContainElement(HaveField("Name", renovate.VolumeRenovateTmp)))
			Expect(mounts).To(ContainElement(HaveField("MountPath", "/tmp")))
		})

		It("should apply ImagePullSecrets from RenovateConfig", func() {
			renovateCR.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
				{Name: "renovate-registry-secret"},
			}

			jobSpec := &batchv1.JobSpec{}
			renovate.DefaultJobSpec(jobSpec, renovateCR, renovateCM)

			Expect(jobSpec.Template.Spec.ImagePullSecrets).To(HaveLen(1))
			Expect(jobSpec.Template.Spec.ImagePullSecrets[0].Name).To(Equal("renovate-registry-secret"))
		})

		It("should merge ImagePullSecrets from RenovateConfig and WithImagePullSecrets", func() {
			renovateCR.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
				{Name: "config-secret"},
			}

			jobSpec := &batchv1.JobSpec{}
			renovate.DefaultJobSpec(
				jobSpec, renovateCR, renovateCM,
				renovate.WithImagePullSecrets([]corev1.LocalObjectReference{
					{Name: "extra-secret"},
				}),
			)

			Expect(jobSpec.Template.Spec.ImagePullSecrets).To(HaveLen(2))
			Expect(jobSpec.Template.Spec.ImagePullSecrets[0].Name).To(Equal("config-secret"))
			Expect(jobSpec.Template.Spec.ImagePullSecrets[1].Name).To(Equal("extra-secret"))
		})

		DescribeTable(
			"Functional Options",
			func(opts []renovate.JobOption, validator func(*batchv1.JobSpec)) {
				jobSpec := &batchv1.JobSpec{}
				renovate.DefaultJobSpec(jobSpec, renovateCR, renovateCM, opts...)
				validator(jobSpec)
			},
			Entry(
				"WithRepository",
				[]renovate.JobOption{renovate.WithRepository("org/repo")},
				func(spec *batchv1.JobSpec) {
					env := spec.Template.Spec.Containers[0].Env
					Expect(env).To(ContainElement(HaveField("Name", "RENOVATE_REPOSITORIES")))
					Expect(env).To(ContainElement(HaveField("Value", "org/repo")))
				},
			),
			Entry(
				"WithInitContainer",
				[]renovate.JobOption{renovate.WithInitContainer(corev1.Container{Name: "init", Image: "busybox"})},
				func(spec *batchv1.JobSpec) {
					Expect(spec.Template.Spec.InitContainers).To(HaveLen(1))
					Expect(spec.Template.Spec.InitContainers[0].Name).To(Equal("init"))
				},
			),
			Entry(
				"WithExtraVolumes",
				[]renovate.JobOption{renovate.WithExtraVolumes(containers.WithEmptyDirVolume("extra"))},
				func(spec *batchv1.JobSpec) {
					Expect(spec.Template.Spec.Volumes).To(HaveLen(3))
					Expect(spec.Template.Spec.Volumes).To(ContainElement(HaveField("Name", "extra")))
				},
			),
			Entry(
				"WithExtraEnv",
				[]renovate.JobOption{renovate.WithExtraEnv([]corev1.EnvVar{{Name: "FOO", Value: "BAR"}})},
				func(spec *batchv1.JobSpec) {
					env := spec.Template.Spec.Containers[0].Env
					Expect(env).To(ContainElement(HaveField("Name", "FOO")))
					Expect(env).To(ContainElement(HaveField("Value", "BAR")))
				},
			),
			Entry(
				"WithImagePullSecrets",
				[]renovate.JobOption{renovate.WithImagePullSecrets([]corev1.LocalObjectReference{
					{Name: "extra-registry-secret"},
				})},
				func(spec *batchv1.JobSpec) {
					Expect(spec.Template.Spec.ImagePullSecrets).To(ContainElement(HaveField("Name", "extra-registry-secret")))
				},
			),
			Entry(
				"WithPodSpec NodeSelector",
				[]renovate.JobOption{renovate.WithPodSpec(renovatev1beta1.PodSpec{
					NodeSelector: map[string]string{"disktype": "ssd"},
				})},
				func(spec *batchv1.JobSpec) {
					Expect(spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("disktype", "ssd"))
				},
			),
			Entry(
				"WithPodSpec Affinity",
				[]renovate.JobOption{renovate.WithPodSpec(renovatev1beta1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{MatchExpressions: []corev1.NodeSelectorRequirement{
										{Key: "kubernetes.io/e2e-az-name", Operator: corev1.NodeSelectorOpIn, Values: []string{"e2e-az1", "e2e-az2"}},
									}},
								},
							},
						},
					},
				})},
				func(spec *batchv1.JobSpec) {
					Expect(spec.Template.Spec.Affinity).NotTo(BeNil())
					Expect(spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil())
				},
			),
			Entry(
				"WithPodSpec Tolerations",
				[]renovate.JobOption{renovate.WithPodSpec(renovatev1beta1.PodSpec{
					Tolerations: []corev1.Toleration{
						{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "value1", Effect: corev1.TaintEffectNoSchedule},
					},
				})},
				func(spec *batchv1.JobSpec) {
					Expect(spec.Template.Spec.Tolerations).To(HaveLen(1))
					Expect(spec.Template.Spec.Tolerations[0].Key).To(Equal("key1"))
				},
			),
			Entry(
				"WithPodSpec TopologySpreadConstraints",
				[]renovate.JobOption{renovate.WithPodSpec(renovatev1beta1.PodSpec{
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
						{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.DoNotSchedule},
					},
				})},
				func(spec *batchv1.JobSpec) {
					Expect(spec.Template.Spec.TopologySpreadConstraints).To(HaveLen(1))
					Expect(spec.Template.Spec.TopologySpreadConstraints[0].TopologyKey).To(Equal("zone"))
				},
			),
			Entry(
				"WithPodSpec Multiple Fields",
				[]renovate.JobOption{renovate.WithPodSpec(renovatev1beta1.PodSpec{
					NodeSelector: map[string]string{"disktype": "ssd"},
					Tolerations: []corev1.Toleration{
						{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "value1"},
					},
				})},
				func(spec *batchv1.JobSpec) {
					Expect(spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("disktype", "ssd"))
					Expect(spec.Template.Spec.Tolerations).To(HaveLen(1))
				},
			),
			Entry(
				"WithResources",
				[]renovate.JobOption{renovate.WithResources(corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				})},
				func(spec *batchv1.JobSpec) {
					container := spec.Template.Spec.Containers[0]
					Expect(container.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
					Expect(container.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("128Mi")))
					Expect(container.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("500m")))
					Expect(container.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("512Mi")))
				},
			),
			Entry(
				"WithSecurityContext",
				[]renovate.JobOption{renovate.WithSecurityContext(&corev1.SecurityContext{
					RunAsNonRoot: new(true),
					RunAsUser:    new(int64(1000)),
				})},
				func(spec *batchv1.JobSpec) {
					container := spec.Template.Spec.Containers[0]
					Expect(container.SecurityContext).NotTo(BeNil())
					Expect(*container.SecurityContext.RunAsNonRoot).To(BeTrue())
					Expect(*container.SecurityContext.RunAsUser).To(Equal(int64(1000)))
				},
			),
			Entry(
				"WithPodSpec Resources",
				[]renovate.JobOption{renovate.WithPodSpec(renovatev1beta1.PodSpec{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("200m"),
						},
					},
				})},
				func(spec *batchv1.JobSpec) {
					container := spec.Template.Spec.Containers[0]
					Expect(container.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("200m")))
				},
			),
			Entry(
				"WithPodSpec SecurityContext",
				[]renovate.JobOption{renovate.WithPodSpec(renovatev1beta1.PodSpec{
					SecurityContext: &corev1.SecurityContext{
						ReadOnlyRootFilesystem: new(true),
					},
				})},
				func(spec *batchv1.JobSpec) {
					container := spec.Template.Spec.Containers[0]
					Expect(container.SecurityContext).NotTo(BeNil())
					Expect(*container.SecurityContext.ReadOnlyRootFilesystem).To(BeTrue())
				},
			),
			Entry(
				"WithPodSpec RuntimeClassName",
				[]renovate.JobOption{renovate.WithPodSpec(renovatev1beta1.PodSpec{
					RuntimeClassName: new("gvisor"),
				})},
				func(spec *batchv1.JobSpec) {
					Expect(spec.Template.Spec.RuntimeClassName).NotTo(BeNil())
					Expect(*spec.Template.Spec.RuntimeClassName).To(Equal("gvisor"))
				},
			),
			Entry(
				"WithPodSpec PodAnnotations",
				[]renovate.JobOption{renovate.WithPodSpec(renovatev1beta1.PodSpec{
					PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
				})},
				func(spec *batchv1.JobSpec) {
					Expect(spec.Template.Annotations).To(HaveKeyWithValue("prometheus.io/scrape", "true"))
				},
			),
			Entry(
				"WithPodSpec ScratchVolume",
				[]renovate.JobOption{renovate.WithPodSpec(renovatev1beta1.PodSpec{
					ScratchVolume: &renovatev1beta1.ScratchVolumeSpec{
						Enabled:   true,
						Path:      "/scratch",
						Medium:    corev1.StorageMediumMemory,
						SizeLimit: new(resource.MustParse("1Gi")),
					},
				})},
				func(spec *batchv1.JobSpec) {
					var scratchVol *corev1.Volume

					for i := range spec.Template.Spec.Volumes {
						if spec.Template.Spec.Volumes[i].Name == renovate.VolumeRenovateTmp {
							scratchVol = &spec.Template.Spec.Volumes[i]

							break
						}
					}

					Expect(scratchVol).NotTo(BeNil())
					Expect(scratchVol.EmptyDir).NotTo(BeNil())
					Expect(scratchVol.EmptyDir.Medium).To(Equal(corev1.StorageMediumMemory))
					Expect(scratchVol.EmptyDir.SizeLimit).To(Equal(new(resource.MustParse("1Gi"))))

					env := spec.Template.Spec.Containers[0].Env
					Expect(env).To(ContainElement(HaveField("Name", "RENOVATE_BASE_DIR")))
					Expect(env).To(ContainElement(HaveField("Value", "/scratch")))

					mounts := spec.Template.Spec.Containers[0].VolumeMounts
					Expect(mounts).To(ContainElement(HaveField("Name", renovate.VolumeRenovateTmp)))
					Expect(mounts).To(ContainElement(HaveField("MountPath", "/scratch")))
				},
			),
			Entry(
				"WithPodSpec ScratchVolume Disabled",
				[]renovate.JobOption{renovate.WithPodSpec(renovatev1beta1.PodSpec{
					ScratchVolume: &renovatev1beta1.ScratchVolumeSpec{
						Enabled: false,
					},
				})},
				func(spec *batchv1.JobSpec) {
					var scratchVol *corev1.Volume

					for i := range spec.Template.Spec.Volumes {
						if spec.Template.Spec.Volumes[i].Name == renovate.VolumeRenovateTmp {
							scratchVol = &spec.Template.Spec.Volumes[i]

							break
						}
					}

					Expect(scratchVol).To(BeNil())

					env := spec.Template.Spec.Containers[0].Env
					Expect(env).NotTo(ContainElement(HaveField("Name", "RENOVATE_BASE_DIR")))

					mounts := spec.Template.Spec.Containers[0].VolumeMounts
					Expect(mounts).NotTo(ContainElement(HaveField("Name", renovate.VolumeRenovateTmp)))
				},
			),
			Entry(
				"WithPodSpec ScratchVolume Ephemeral",
				[]renovate.JobOption{renovate.WithPodSpec(renovatev1beta1.PodSpec{
					ScratchVolume: &renovatev1beta1.ScratchVolumeSpec{
						Enabled: true,
						Path:    "/ephemeral",
						Ephemeral: &corev1.EphemeralVolumeSource{
							VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
								Spec: corev1.PersistentVolumeClaimSpec{
									AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								},
							},
						},
					},
				})},
				func(spec *batchv1.JobSpec) {
					var scratchVol *corev1.Volume

					for i := range spec.Template.Spec.Volumes {
						if spec.Template.Spec.Volumes[i].Name == renovate.VolumeRenovateTmp {
							scratchVol = &spec.Template.Spec.Volumes[i]

							break
						}
					}

					Expect(scratchVol).NotTo(BeNil())
					Expect(scratchVol.Ephemeral).NotTo(BeNil())
					Expect(scratchVol.EmptyDir).To(BeNil())

					env := spec.Template.Spec.Containers[0].Env
					Expect(env).To(ContainElement(HaveField("Name", "RENOVATE_BASE_DIR")))
					Expect(env).To(ContainElement(HaveField("Value", "/ephemeral")))
				},
			),
			Entry(
				"Multiple Options Combined",
				[]renovate.JobOption{
					renovate.WithRepository("org/repo"),
					renovate.WithExtraEnv([]corev1.EnvVar{{Name: "A", Value: "B"}}),
				},
				func(spec *batchv1.JobSpec) {
					env := spec.Template.Spec.Containers[0].Env
					Expect(env).To(ContainElement(HaveField("Value", "org/repo")))
					Expect(env).To(ContainElement(HaveField("Name", "A")))
				},
			),
		)
	})

	Describe("Controller Helpers", func() {
		var (
			ctx        context.Context
			fakeClient client.Client
			labels     map[string]string
			namespace  string
		)

		BeforeEach(func() {
			ctx = context.Background()
			namespace = "test-ns"
			labels = map[string]string{"app": "renovate-test"}
		})

		DescribeTable(
			"GetActiveJobs",
			func(jobs []*batchv1.Job, expectedCount int) {
				objs := make([]client.Object, len(jobs))
				for i, j := range jobs {
					objs[i] = j
				}

				fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objs...).Build()

				active, err := renovate.GetActiveJobs(ctx, fakeClient, namespace, labels)
				Expect(err).NotTo(HaveOccurred())
				Expect(active).To(HaveLen(expectedCount))
			},
			Entry("Found 1 active job", []*batchv1.Job{newJob("j1", "active")}, 1),
			Entry("Ignores succeeded jobs", []*batchv1.Job{newJob("j1", "succeeded")}, 0),
			Entry("Ignores failed jobs", []*batchv1.Job{newJob("j1", "failed")}, 0),
			Entry("Mixed states", []*batchv1.Job{
				newJob("active", "active"),
				newJob("done", "succeeded"),
			}, 1),
		)
	})
})
