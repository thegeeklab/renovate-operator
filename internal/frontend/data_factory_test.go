package frontend

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("DataFactory", func() {
	var (
		client      client.Client
		dataFactory *DataFactory
		scheme      *runtime.Scheme
		testObjects []runtime.Object
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		err := renovatev1beta1.AddToScheme(scheme)
		Expect(err).NotTo(HaveOccurred())

		testObjects = []runtime.Object{
			&renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-renovator",
					Namespace:         "test-namespace",
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
				Status: renovatev1beta1.RenovatorStatus{
					Ready: true,
				},
			},
			&renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-repo",
					Namespace:         "test-namespace",
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
				Spec: renovatev1beta1.GitRepoSpec{
					WebhookID: "12345",
				},
				Status: renovatev1beta1.GitRepoStatus{
					Ready: true,
				},
			},
			&renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-runner",
					Namespace:         "test-namespace",
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
				Spec: renovatev1beta1.RunnerSpec{
					Instances: 1,
				},
				Status: renovatev1beta1.RunnerStatus{
					Ready: true,
				},
			},
			&renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-discovery",
					Namespace:         "test-namespace",
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
				Status: renovatev1beta1.DiscoveryStatus{
					Ready: true,
				},
			},
		}

		client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(testObjects...).Build()
		dataFactory = NewDataFactory(client)
	})

	Describe("NewDataFactory", func() {
		It("should create a new DataFactory", func() {
			Expect(dataFactory).NotTo(BeNil())
		})
	})

	Describe("GetRenovators", func() {
		It("should return a list of renovators", func() {
			renovators, err := dataFactory.GetRenovators(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(renovators).To(HaveLen(1))
			Expect(renovators[0].Name).To(Equal("test-renovator"))
			Expect(renovators[0].Namespace).To(Equal("test-namespace"))
			Expect(renovators[0].Ready).To(BeTrue())
		})
	})

	Describe("GetGitRepos", func() {
		It("should return a list of git repos", func() {
			repos, err := dataFactory.GetGitRepos(context.Background(), "", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveLen(1))
			Expect(repos[0].Name).To(Equal("test-repo"))
			Expect(repos[0].Namespace).To(Equal("test-namespace"))
			Expect(repos[0].WebhookID).To(Equal("12345"))
			Expect(repos[0].Ready).To(BeTrue())
		})

		It("should filter by namespace", func() {
			repos, err := dataFactory.GetGitRepos(context.Background(), "test-namespace", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveLen(1))
		})

		It("should return empty list for non-matching namespace", func() {
			repos, err := dataFactory.GetGitRepos(context.Background(), "non-existent", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(BeEmpty())
		})
	})

	Describe("GetRunners", func() {
		It("should return a list of runners", func() {
			runners, err := dataFactory.GetRunners(context.Background(), "", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(runners).To(HaveLen(1))
			Expect(runners[0].Name).To(Equal("test-runner"))
			Expect(runners[0].Namespace).To(Equal("test-namespace"))
			Expect(runners[0].Instances).To(Equal(int32(1)))
			Expect(runners[0].Ready).To(BeTrue())
		})

		It("should filter by namespace", func() {
			runners, err := dataFactory.GetRunners(context.Background(), "test-namespace", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(runners).To(HaveLen(1))
		})

		It("should return empty list for non-matching namespace", func() {
			runners, err := dataFactory.GetRunners(context.Background(), "non-existent", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(runners).To(BeEmpty())
		})
	})

	Describe("GetDiscoveries", func() {
		It("should return a list of discoveries", func() {
			// Call the method
			discoveries, err := dataFactory.GetDiscoveries(context.Background(), "", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(discoveries).To(HaveLen(1))
			Expect(discoveries[0].Name).To(Equal("test-discovery"))
			Expect(discoveries[0].Namespace).To(Equal("test-namespace"))
			Expect(discoveries[0].Ready).To(BeTrue())
		})

		It("should filter by namespace", func() {
			// Call the method with namespace filter
			discoveries, err := dataFactory.GetDiscoveries(context.Background(), "test-namespace", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(discoveries).To(HaveLen(1))
		})

		It("should return empty list for non-matching namespace", func() {
			discoveries, err := dataFactory.GetDiscoveries(context.Background(), "non-existent", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(discoveries).To(BeEmpty())
		})
	})
})
