package frontend

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("DataFactory", func() {
	var (
		fakeClient  client.Client
		dataFactory *DataFactory
		scheme      *runtime.Scheme
		testObjects []runtime.Object
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()

		err := renovatev1beta1.AddToScheme(scheme)
		Expect(err).NotTo(HaveOccurred())

		err = batchv1.AddToScheme(scheme)
		Expect(err).NotTo(HaveOccurred())

		testObjects = []runtime.Object{
			&renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-renovator",
					Namespace:         "test-namespace",
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
			},
			&renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo-b",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "test-renovator",
					},
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
			},
			&renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo-a",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "other-renovator",
					},
					CreationTimestamp: metav1.NewTime(time.Now().Add(1 * time.Hour)),
				},
			},
			&renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "test-renovator",
					},
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
			},
			&renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "test-renovator",
					},
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
			},
			&batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-job-1",
					Namespace:         "test-namespace",
					CreationTimestamp: metav1.NewTime(time.Now()),
					Labels: map[string]string{
						renovatev1beta1.LabelGitRepo:     "test-repo-b",
						renovatev1beta1.LabelAppInstance: "test-runner",
					},
				},
				Status: batchv1.JobStatus{
					Succeeded:      1,
					CompletionTime: &metav1.Time{Time: time.Now()},
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
					},
				},
			},
		}

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(testObjects...).Build()
		fakeClientset := kubernetesfake.NewClientset()

		dataFactory = NewDataFactory(fakeClient, fakeClientset)
	})

	Describe("GetRenovators", func() {
		It("should return a list of renovators", func() {
			renovators, err := dataFactory.GetRenovators(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(renovators).To(HaveLen(1))
			Expect(renovators[0].Name).To(Equal("test-renovator"))
		})

		It("should not apply the Renovator label filter to itself", func() {
			opts := ListOptions{Renovator: "test-renovator"}
			renovators, err := dataFactory.GetRenovators(context.Background(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(renovators).To(HaveLen(1))
		})
	})

	Describe("GetGitRepos", func() {
		It("should return all git repos when no options are provided", func() {
			repos, err := dataFactory.GetGitRepos(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveLen(2))
		})

		It("should correctly filter git repos by Renovator label", func() {
			opts := ListOptions{Renovator: "test-renovator"}
			repos, err := dataFactory.GetGitRepos(context.Background(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveLen(1))
			Expect(repos[0].Name).To(Equal("test-repo-b"))
		})
	})

	Describe("GetRunners", func() {
		It("should correctly filter runners by Renovator label", func() {
			opts := ListOptions{Renovator: "test-renovator"}
			runners, err := dataFactory.GetRunners(context.Background(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(runners).To(HaveLen(1))
			Expect(runners[0].Name).To(Equal("test-runner"))
		})
	})

	Describe("GetDiscoveries", func() {
		It("should correctly filter discoveries by Renovator label", func() {
			opts := ListOptions{Renovator: "test-renovator"}
			discoveries, err := dataFactory.GetDiscoveries(context.Background(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(discoveries).To(HaveLen(1))
			Expect(discoveries[0].Name).To(Equal("test-discovery"))
		})
	})

	Describe("GetJobsForRepo", func() {
		It("should return a list of jobs matching the git repo", func() {
			opts := ListOptions{Namespace: "test-namespace"}
			jobs, err := dataFactory.GetJobsForRepo(context.Background(), "test-repo-b", opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(jobs).To(HaveLen(1))

			Expect(jobs[0].Name).To(Equal("test-job-1"))
			Expect(jobs[0].Namespace).To(Equal("test-namespace"))
			Expect(jobs[0].Runner).To(Equal("test-runner"))
			Expect(jobs[0].Status).To(Equal("Succeeded"))
		})

		It("should return empty list for non-matching repo", func() {
			opts := ListOptions{Namespace: "test-namespace"}
			jobs, err := dataFactory.GetJobsForRepo(context.Background(), "missing", opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(jobs).To(BeEmpty())
		})
	})

	Describe("GetJobLogs", func() {
		It("should return an error if no pods are found for the job", func() {
			_, err := dataFactory.GetJobLogs(context.Background(), "test-namespace", "test-job-1")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no pods found for job: test-job-1"))
		})
	})
})
