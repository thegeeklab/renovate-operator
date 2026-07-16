package frontend

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/frontend/viewmodel"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
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

		dataFactory = NewDataFactory(fakeClient, fakeClientset, nil)
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

		It("should populate LastRenovateAt from GitRepo status when completed", func() {
			jobTime := time.Now().Add(-2 * time.Hour)
			completedTime := time.Now().Add(-1 * time.Hour)
			repoWithStatus := &renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "repo-with-lastrun",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "test-renovator",
					},
					CreationTimestamp: metav1.NewTime(jobTime),
				},
				Status: renovatev1beta1.GitRepoStatus{
					LastRenovateTime: &metav1.Time{Time: jobTime},
					Conditions: []metav1.Condition{
						{
							Type:               renovatev1beta1.GitRepoConditionRenovateCompleted,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.NewTime(completedTime),
						},
					},
				},
			}
			Expect(fakeClient.Create(context.Background(), repoWithStatus)).To(Succeed())

			repos, err := dataFactory.GetGitRepos(context.Background())
			Expect(err).NotTo(HaveOccurred())

			var foundRepo *viewmodel.GitRepoInfo

			for i := range repos {
				if repos[i].Name == "repo-with-lastrun" {
					foundRepo = &repos[i]

					break
				}
			}

			Expect(foundRepo).NotTo(BeNil())
			Expect(foundRepo.LastRenovateAt.IsZero()).To(BeFalse())
			Expect(foundRepo.LastRenovateAt.Unix()).To(BeNumerically("~", jobTime.Unix(), 2))
			Expect(foundRepo.LastRenovateStatus).To(Equal(viewmodel.StatusSucceeded))
		})

		It("should return empty LastRenovateAt when no jobs exist", func() {
			repos, err := dataFactory.GetGitRepos(context.Background())
			Expect(err).NotTo(HaveOccurred())

			for _, repo := range repos {
				if repo.Name != "repo-with-lastrun" {
					Expect(repo.LastRenovateAt.IsZero()).To(BeTrue())
				}
			}
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
			Expect(jobs[0].Status).To(Equal(viewmodel.StatusSucceeded))
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

	Describe("GetPRActivityForRenovator", func() {
		It("returns an empty summary without required params", func() {
			summary, err := dataFactory.GetPRActivityForRenovator(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(summary.Open).To(BeZero())
			Expect(summary.HasRecentData).To(BeFalse())
		})

		It("returns zero counts with no recent data when no jobs have logs", func() {
			summary, err := dataFactory.GetPRActivityForRenovator(
				context.Background(),
				ListOptions{Namespace: "test-namespace", Renovator: "test-renovator"},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(summary.Open).To(BeZero())
			Expect(summary.NeedsApproval).To(BeZero())
			Expect(summary.HasRecentData).To(BeFalse())
		})
	})

	Describe("isJobTerminal", func() {
		It("returns false for a fresh job", func() {
			job := &batchv1.Job{Status: batchv1.JobStatus{}}
			Expect(isJobTerminal(job)).To(BeFalse())
		})

		It("returns true when CompletionTime is set", func() {
			now := metav1.Now()
			job := &batchv1.Job{Status: batchv1.JobStatus{CompletionTime: &now}}
			Expect(isJobTerminal(job)).To(BeTrue())
		})

		It("returns true when JobComplete condition is True", func() {
			job := &batchv1.Job{Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
				},
			}}
			Expect(isJobTerminal(job)).To(BeTrue())
		})

		It("returns true when JobFailed condition is True", func() {
			job := &batchv1.Job{Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
				},
			}}
			Expect(isJobTerminal(job)).To(BeTrue())
		})
	})

	Describe("findLatestTerminalJobsByRepo", func() {
		It("returns an empty map when no jobs exist", func() {
			jobs, err := dataFactory.findLatestTerminalJobsByRepo(
				context.Background(), "test-namespace", "test-renovator",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(jobs).To(BeEmpty())
		})

		It("returns the most recent completed job per repo", func() {
			older := metav1.NewTime(time.Now().Add(-1 * time.Hour))
			newer := metav1.NewTime(time.Now().Add(-1 * time.Minute))

			jobs := []client.Object{
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo-a-older",
						Namespace: "test-namespace",
						Labels: map[string]string{
							renovatev1beta1.LabelRenovator: "test-renovator",
							renovatev1beta1.LabelGitRepo:   "repo-a",
						},
					},
					Status: batchv1.JobStatus{CompletionTime: &older},
				},
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo-a-newer",
						Namespace: "test-namespace",
						Labels: map[string]string{
							renovatev1beta1.LabelRenovator: "test-renovator",
							renovatev1beta1.LabelGitRepo:   "repo-a",
						},
					},
					Status: batchv1.JobStatus{CompletionTime: &newer},
				},
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo-b-only",
						Namespace: "test-namespace",
						Labels: map[string]string{
							renovatev1beta1.LabelRenovator: "test-renovator",
							renovatev1beta1.LabelGitRepo:   "repo-b",
						},
					},
					Status: batchv1.JobStatus{CompletionTime: &newer},
				},
			}
			Expect(fakeClient.Create(context.Background(), jobs[0])).To(Succeed())
			Expect(fakeClient.Create(context.Background(), jobs[1])).To(Succeed())
			Expect(fakeClient.Create(context.Background(), jobs[2])).To(Succeed())

			latest, err := dataFactory.findLatestTerminalJobsByRepo(
				context.Background(), "test-namespace", "test-renovator",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(latest).To(HaveLen(2))
			Expect(latest["repo-a"].Name).To(Equal("repo-a-newer"))
			Expect(latest["repo-b"].Name).To(Equal("repo-b-only"))
		})

		It("keys the map by the same label value the runner writes (truncated/hashed for long names)", func() {
			// Reproduces the bug where jobs are labeled with k8s.SanitizeLabel(repo.Name)
			// (a 63-char DNS-1035 normalization) but the aggregator was looking them
			// up by the un-normalized repo.Name, missing any GitRepo whose name
			// exceeded 63 characters.
			longName := "this-is-a-very-long-repo-name-that-exceeds-the-63-character-dns-label-limit-yes"
			Expect(len(longName)).To(BeNumerically(">", 63))

			normalized, err := k8s.SanitizeLabel(longName)
			Expect(err).NotTo(HaveOccurred())
			Expect(normalized).NotTo(Equal(longName))

			now := metav1.NewTime(time.Now().Add(-1 * time.Minute))
			Expect(fakeClient.Create(context.Background(), &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "long-repo-job",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "test-renovator",
						renovatev1beta1.LabelGitRepo:   normalized,
					},
				},
				Status: batchv1.JobStatus{CompletionTime: &now},
			})).To(Succeed())

			latest, err := dataFactory.findLatestTerminalJobsByRepo(
				context.Background(), "test-namespace", "test-renovator",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(latest).To(HaveLen(1))
			Expect(latest[normalized].Name).To(Equal("long-repo-job"))
		})
	})

	Describe("GetPRActivityForRenovator cache", func() {
		It("returns the same summary for repeated calls without re-listing jobs", func() {
			now := metav1.NewTime(time.Now().Add(-1 * time.Minute))
			Expect(fakeClient.Create(context.Background(), &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cached-job",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "test-renovator",
						renovatev1beta1.LabelGitRepo:   "test-repo-a",
					},
				},
				Status: batchv1.JobStatus{CompletionTime: &now},
			})).To(Succeed())

			ctx := context.Background()
			opts := ListOptions{Namespace: "test-namespace", Renovator: "test-renovator"}

			first, err := dataFactory.GetPRActivityForRenovator(ctx, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(context.Background(), &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cached-job",
					Namespace: "test-namespace",
				},
			})).To(Succeed())

			second, err := dataFactory.GetPRActivityForRenovator(ctx, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(second).To(Equal(first))
		})
	})
})
