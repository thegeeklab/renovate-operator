package renovate_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	containers "github.com/thegeeklab/renovate-operator/internal/resource/container"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
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

func newDatedJob(name string, ageHours int, status string) *batchv1.Job {
	j := newJob(name, status)
	j.CreationTimestamp = metav1.NewTime(time.Now().Add(time.Duration(-ageHours) * time.Hour))

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

			Expect(jobSpec.CompletionMode).To(Equal(ptr.To(batchv1.NonIndexedCompletion)))
			Expect(jobSpec.Parallelism).To(Equal(ptr.To(int32(1))))
			Expect(jobSpec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))
			Expect(jobSpec.Template.Spec.Volumes).To(HaveLen(2))
			Expect(jobSpec.Template.Spec.Containers).To(HaveLen(1))
		})

		DescribeTable("Functional Options",
			func(opts []renovate.JobOption, validator func(*batchv1.JobSpec)) {
				jobSpec := &batchv1.JobSpec{}
				renovate.DefaultJobSpec(jobSpec, renovateCR, renovateCM, opts...)
				validator(jobSpec)
			},
			Entry("WithRepository",
				[]renovate.JobOption{renovate.WithRepository("org/repo")},
				func(spec *batchv1.JobSpec) {
					env := spec.Template.Spec.Containers[0].Env
					Expect(env).To(ContainElement(HaveField("Name", "RENOVATE_REPOSITORIES")))
					Expect(env).To(ContainElement(HaveField("Value", "org/repo")))
				},
			),
			Entry("WithInitContainer",
				[]renovate.JobOption{renovate.WithInitContainer(corev1.Container{Name: "init", Image: "busybox"})},
				func(spec *batchv1.JobSpec) {
					Expect(spec.Template.Spec.InitContainers).To(HaveLen(1))
					Expect(spec.Template.Spec.InitContainers[0].Name).To(Equal("init"))
				},
			),
			Entry("WithExtraVolumes",
				[]renovate.JobOption{renovate.WithExtraVolumes(containers.WithEmptyDirVolume("extra"))},
				func(spec *batchv1.JobSpec) {
					Expect(spec.Template.Spec.Volumes).To(HaveLen(3))
					Expect(spec.Template.Spec.Volumes).To(ContainElement(HaveField("Name", "extra")))
				},
			),
			Entry("WithExtraEnv",
				[]renovate.JobOption{renovate.WithExtraEnv([]corev1.EnvVar{{Name: "FOO", Value: "BAR"}})},
				func(spec *batchv1.JobSpec) {
					env := spec.Template.Spec.Containers[0].Env
					Expect(env).To(ContainElement(HaveField("Name", "FOO")))
					Expect(env).To(ContainElement(HaveField("Value", "BAR")))
				},
			),
			Entry("Multiple Options Combined",
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
			ctx       context.Context
			k8sClient client.Client
			labels    map[string]string
			namespace string
		)

		BeforeEach(func() {
			ctx = context.Background()
			namespace = "test-ns"
			labels = map[string]string{"app": "renovate-test"}
		})

		DescribeTable("GetActiveJobs",
			func(jobs []*batchv1.Job, expectedCount int) {
				objs := make([]client.Object, len(jobs))
				for i, j := range jobs {
					objs[i] = j
				}
				k8sClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objs...).Build()

				active, err := renovate.GetActiveJobs(ctx, k8sClient, namespace, labels)
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

		DescribeTable("PruneJobHistory",
			func(jobs []*batchv1.Job, limits [2]int, expectedNames []string) {
				objs := make([]client.Object, len(jobs))
				for i, j := range jobs {
					objs[i] = j
				}
				k8sClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objs...).Build()

				err := renovate.PruneJobHistory(ctx, k8sClient, namespace, labels, limits[0], limits[1])
				Expect(err).NotTo(HaveOccurred())

				remaining := &batchv1.JobList{}
				err = k8sClient.List(ctx, remaining, client.InNamespace(namespace))
				Expect(err).NotTo(HaveOccurred())

				var names []string
				for _, j := range remaining.Items {
					names = append(names, j.Name)
				}
				Expect(names).To(ConsistOf(expectedNames))
			},
			Entry("Deletes old successful jobs",
				[]*batchv1.Job{
					newDatedJob("new", 0, "succeeded"),
					newDatedJob("old", 1, "succeeded"),
					newDatedJob("oldest", 2, "succeeded"),
				},
				[2]int{2, 1},
				[]string{"new", "old"},
			),
			Entry("Deletes old failed jobs",
				[]*batchv1.Job{
					newDatedJob("fail-new", 0, "failed"),
					newDatedJob("fail-old", 2, "failed"),
				},
				[2]int{3, 1},
				[]string{"fail-new"},
			),
			Entry("Preserves active jobs regardless of limits",
				[]*batchv1.Job{
					newDatedJob("active-old", 5, "active"),
					newDatedJob("success-new", 0, "succeeded"),
				},
				[2]int{0, 0},
				[]string{"active-old"},
			),
		)
	})
})
