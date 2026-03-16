package runner

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	fakeclock "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileJob", func() {
	var (
		fakeClient client.Client
		reconciler *Reconciler
		instance   *renovatev1beta1.Runner
		renovate   *renovatev1beta1.RenovateConfig
		repo1      *renovatev1beta1.GitRepo
		repo2      *renovatev1beta1.GitRepo
		ctx        context.Context
		scheme     *runtime.Scheme
		now        time.Time
		fakeClock  *fakeclock.FakeClock
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())

		instance = &renovatev1beta1.Runner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runner",
				Namespace: "default",
				Labels: map[string]string{
					renovatev1beta1.RenovatorLabel: "renovator-id",
				},
			},
			Spec: renovatev1beta1.RunnerSpec{
				JobSpec: renovatev1beta1.JobSpec{
					Schedule: "*/5 * * * *",
				},
			},
		}
		rr := &RunnerCustomDefaulter{}
		Expect(rr.Default(ctx, instance)).To(Succeed())

		renovate = &renovatev1beta1.RenovateConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-renovate",
				Namespace: "default",
			},
			Spec: renovatev1beta1.RenovateConfigSpec{
				ImageSpec: renovatev1beta1.ImageSpec{
					Image:           "renovate/renovate:latest",
					ImagePullPolicy: corev1.PullAlways,
				},
				Platform: renovatev1beta1.PlatformSpec{
					Type: "github",
				},
			},
		}
		rd := &RenovateConfigCustomDefaulter{}
		Expect(rd.Default(ctx, renovate)).To(Succeed())

		// Create two GitRepos for runner specific tests
		repo1 = &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "repo-1",
				Namespace: instance.Namespace,
			},
			Spec: renovatev1beta1.GitRepoSpec{
				Name: "test/repo-1",
			},
		}
		repo2 = &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "repo-2",
				Namespace: instance.Namespace,
			},
			Spec: renovatev1beta1.GitRepoSpec{
				Name: "test/repo-2",
			},
		}

		now = time.Date(2026, 2, 27, 15, 0, 0, 0, time.UTC)
		fakeClock = fakeclock.NewFakeClock(now)

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(instance, renovate, repo1, repo2).
			WithStatusSubresource(instance).
			Build()

		reconciler = &Reconciler{
			Client:    fakeClient,
			scheme:    scheme,
			scheduler: scheduler.NewManager(fakeClient, scheme, fakeClock),
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: instance.Namespace,
					Name:      instance.Name,
				},
			},
			instance: instance,
			renovate: renovate,
		}
	})

	Describe("reconcileJob", func() {
		expectedLabels := func(repoName string) map[string]string {
			expected := RunnerLabels(reconciler.req)

			if val, ok := instance.Labels[renovatev1beta1.RenovatorLabel]; ok {
				expected[renovatev1beta1.RenovatorLabel] = val
			}

			if repoName != "" {
				expected["renovate.thegeeklab.de/gitrepo"] = repoName
			}

			return expected
		}

		Context("when runner is suspended", func() {
			BeforeEach(func() {
				suspended := true
				instance.Spec.Suspend = &suspended
				Expect(fakeClient.Update(ctx, instance)).To(Succeed())
			})

			It("should skip job creation", func() {
				result, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(&ctrl.Result{}))

				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
				Expect(jobList.Items).To(BeEmpty())
			})
		})

		Context("when runner is suspended but globally manually triggered", func() {
			BeforeEach(func() {
				suspended := true
				instance.Spec.Suspend = &suspended
				instance.Annotations = map[string]string{
					"renovate.thegeeklab.de/operation": "renovate",
				}
				Expect(fakeClient.Update(ctx, instance)).To(Succeed())
			})

			It("should create jobs for all repos and remove the runner annotation", func() {
				_, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Verify Jobs Creation (1 for each repo)
				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
				Expect(jobList.Items).To(HaveLen(2))

				// Verify Annotation Removal on Runner
				updatedInstance := &renovatev1beta1.Runner{}
				Expect(fakeClient.Get(ctx, reconciler.req.NamespacedName, updatedInstance)).To(Succeed())
				Expect(updatedInstance.Annotations).NotTo(HaveKey("renovate.thegeeklab.de/operation"))

				// Verify Status Update
				Expect(updatedInstance.Status.LastScheduleTime).NotTo(BeNil())
			})
		})

		Context("when runner is suspended but a specific GitRepo is manually triggered", func() {
			BeforeEach(func() {
				suspended := true
				instance.Spec.Suspend = &suspended
				Expect(fakeClient.Update(ctx, instance)).To(Succeed())

				repo1.Annotations = map[string]string{
					"renovate.thegeeklab.de/operation": "renovate",
				}
				Expect(fakeClient.Update(ctx, repo1)).To(Succeed())
			})

			It("should create a job ONLY for the triggered repo and remove its annotation", func() {
				result, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(&ctrl.Result{}))

				// Verify Job Creation
				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
				Expect(jobList.Items).To(HaveLen(1))

				job := jobList.Items[0]
				Expect(job.GenerateName).To(HavePrefix("repo-1-"))
				Expect(job.Labels).To(Equal(expectedLabels("repo-1")))

				// Verify Annotation Removal on Repo
				updatedRepo := &renovatev1beta1.GitRepo{}
				repoKey := types.NamespacedName{Name: repo1.Name, Namespace: repo1.Namespace}
				Expect(fakeClient.Get(ctx, repoKey, updatedRepo)).To(Succeed())
				Expect(updatedRepo.Annotations).NotTo(HaveKey("renovate.thegeeklab.de/operation"))

				// Verify Runner Status is unaffected
				updatedInstance := &renovatev1beta1.Runner{}
				Expect(fakeClient.Get(ctx, reconciler.req.NamespacedName, updatedInstance)).To(Succeed())
				Expect(updatedInstance.Status.LastScheduleTime).To(BeNil())
			})
		})

		Context("when there is an active job for one of the repos", func() {
			BeforeEach(func() {
				activeJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "active-job-repo-1",
						Namespace: "default",
						Labels:    expectedLabels("repo-1"),
					},
					Status: batchv1.JobStatus{
						Active: 1,
					},
				}
				Expect(fakeClient.Create(ctx, activeJob)).To(Succeed())
			})

			It("should skip the active repo but create a job for the other", func() {
				_, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())

				// 1 pre-existing active job + 1 new job for repo-2
				Expect(jobList.Items).To(HaveLen(2))

				newJobsFound := 0

				for _, job := range jobList.Items {
					if job.Name != "active-job-repo-1" {
						Expect(job.GenerateName).To(HavePrefix("repo-2-"))
						Expect(job.Labels).To(Equal(expectedLabels("repo-2")))

						newJobsFound++
					}
				}

				Expect(newJobsFound).To(Equal(1))
			})
		})

		Context("when job should run globally based on schedule", func() {
			It("should create new jobs for all repos and update status", func() {
				_, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
				Expect(jobList.Items).To(HaveLen(2))

				for _, job := range jobList.Items {
					repoName := job.Labels["renovate.thegeeklab.de/gitrepo"]
					Expect(job.Labels).To(Equal(expectedLabels(repoName)))
				}

				updatedInstance := &renovatev1beta1.Runner{}
				Expect(fakeClient.Get(ctx, reconciler.req.NamespacedName, updatedInstance)).To(Succeed())
				Expect(updatedInstance.Status.LastScheduleTime).NotTo(BeNil())
			})
		})
	})

	Describe("updateJob", func() {
		It("should configure job with correct specifications for a GitRepo", func() {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, repo1, nil)

			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			mainContainer := job.Spec.Template.Spec.Containers[0]
			Expect(mainContainer.Name).To(Equal("renovate"))
			Expect(mainContainer.Image).To(Equal("renovate/renovate:latest"))

			expectedSA := metadata.GenericMetadata(reconciler.req).Name
			Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(expectedSA))
		})
	})
})
