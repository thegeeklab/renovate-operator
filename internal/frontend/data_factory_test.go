package frontend

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/frontend/auth"
	"github.com/thegeeklab/renovate-operator/internal/frontend/auth/mocks"
	"github.com/thegeeklab/renovate-operator/internal/frontend/viewmodel"
	"github.com/thegeeklab/renovate-operator/internal/logreader"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("DataFactory", func() {
	var (
		fakeClient    client.Client
		fakeClientset *kubernetesfake.Clientset
		dataFactory   *DataFactory
		scheme        *runtime.Scheme
		testObjects   []runtime.Object
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()

		err := renovatev1beta1.AddToScheme(scheme)
		Expect(err).NotTo(HaveOccurred())

		err = batchv1.AddToScheme(scheme)
		Expect(err).NotTo(HaveOccurred())

		testObjects = []runtime.Object{
			&renovatev1beta1.Renovator{
				Name:              "test-renovator",
				Namespace:         "test-namespace",
				CreationTimestamp: metav1.NewTime(time.Now()),
			},
			&renovatev1beta1.GitRepo{
				Name:      "test-repo-b",
				Namespace: "test-namespace",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "test-renovator",
				},
				CreationTimestamp: metav1.NewTime(time.Now()),
			},
			&renovatev1beta1.GitRepo{
				Name:      "test-repo-a",
				Namespace: "test-namespace",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "other-renovator",
				},
				CreationTimestamp: metav1.NewTime(time.Now().Add(1 * time.Hour)),
			},
			&renovatev1beta1.Runner{
				Name:      "test-runner",
				Namespace: "test-namespace",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "test-renovator",
				},
				CreationTimestamp: metav1.NewTime(time.Now()),
			},
			&renovatev1beta1.Discovery{
				Name:      "test-discovery",
				Namespace: "test-namespace",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "test-renovator",
				},
				CreationTimestamp: metav1.NewTime(time.Now()),
			},
			&batchv1.Job{
				Name:              "test-job-1",
				Namespace:         "test-namespace",
				CreationTimestamp: metav1.NewTime(time.Now()),
				Labels: map[string]string{
					renovatev1beta1.LabelGitRepo:     "test-repo-b",
					renovatev1beta1.LabelAppInstance: "test-runner",
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
		fakeClientset = kubernetesfake.NewClientset()

		dataFactory = NewDataFactory(fakeClient, fakeClientset, nil, nil)
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
				Name:      "repo-with-lastrun",
				Namespace: "test-namespace",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "test-renovator",
				},
				CreationTimestamp: metav1.NewTime(jobTime),
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

		It("should populate Platform and RepoURL from GitRepo status", func() {
			repoWithPlatform := &renovatev1beta1.GitRepo{
				Name:      "repo-with-platform",
				Namespace: "test-namespace",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "test-renovator",
				},
				CreationTimestamp: metav1.NewTime(time.Now()),
				Spec: renovatev1beta1.GitRepoSpec{
					Name: "testorg/repo-with-platform",
				},
				Status: renovatev1beta1.GitRepoStatus{
					Platform:  "github",
					RepoURL:   "https://github.com/testorg/repo-with-platform",
					WebhookID: "99",
				},
			}
			Expect(fakeClient.Create(context.Background(), repoWithPlatform)).To(Succeed())

			repos, err := dataFactory.GetGitRepos(context.Background())
			Expect(err).NotTo(HaveOccurred())

			var foundRepo *viewmodel.GitRepoInfo

			for i := range repos {
				if repos[i].Name == "repo-with-platform" {
					foundRepo = &repos[i]

					break
				}
			}

			Expect(foundRepo).NotTo(BeNil())
			Expect(foundRepo.Platform).To(Equal("github"))
			Expect(foundRepo.RepoURL).To(Equal("https://github.com/testorg/repo-with-platform"))
			Expect(foundRepo.WebhookID).To(Equal("99"))
		})

		It("should return empty Platform and RepoURL when not set in status", func() {
			repos, err := dataFactory.GetGitRepos(context.Background())
			Expect(err).NotTo(HaveOccurred())

			for _, repo := range repos {
				if repo.Name == "test-repo-a" || repo.Name == "test-repo-b" {
					Expect(repo.Platform).To(BeEmpty())
					Expect(repo.RepoURL).To(BeEmpty())
				}
			}
		})
	})

	Describe("GetGitRepo", func() {
		It("should return a single git repo with all fields populated", func() {
			repoWithPlatform := &renovatev1beta1.GitRepo{
				Name:      "single-repo",
				Namespace: "test-namespace",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "test-renovator",
				},
				CreationTimestamp: metav1.NewTime(time.Now()),
				Spec: renovatev1beta1.GitRepoSpec{
					Name: "testorg/single-repo",
				},
				Status: renovatev1beta1.GitRepoStatus{
					Platform:  "github",
					RepoURL:   "https://github.com/testorg/single-repo",
					WebhookID: "42",
				},
			}
			Expect(fakeClient.Create(context.Background(), repoWithPlatform)).To(Succeed())

			repo, err := dataFactory.GetGitRepo(context.Background(), "test-namespace", "single-repo")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo).NotTo(BeNil())
			Expect(repo.Name).To(Equal("single-repo"))
			Expect(repo.FullName).To(Equal("testorg/single-repo"))
			Expect(repo.Namespace).To(Equal("test-namespace"))
			Expect(repo.Platform).To(Equal("github"))
			Expect(repo.RepoURL).To(Equal("https://github.com/testorg/single-repo"))
			Expect(repo.WebhookID).To(Equal("42"))
			Expect(repo.RenovatorUID).To(Equal("test-renovator"))
		})

		It("should return error when repo does not exist", func() {
			repo, err := dataFactory.GetGitRepo(context.Background(), "test-namespace", "nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(repo).To(BeNil())
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
			dataFactory.logReader = logreader.NewKubernetesReader(fakeClientset)

			_, err := dataFactory.GetJobLogs(context.Background(), "test-namespace", "test-job-1", 0)
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
			Expect(summary.WarnCount).To(BeZero())
			Expect(summary.ErrorCount).To(BeZero())
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
			Expect(summary.WarnCount).To(BeZero())
			Expect(summary.ErrorCount).To(BeZero())
		})
	})

	Describe("GetPerRepoActivity", func() {
		It("returns an empty map without required params", func() {
			perRepo, err := dataFactory.GetPerRepoActivity(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(perRepo).To(BeEmpty())
		})

		It("returns an empty map when no jobs have logs", func() {
			perRepo, err := dataFactory.GetPerRepoActivity(
				context.Background(),
				ListOptions{Namespace: "test-namespace", Renovator: "test-renovator"},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(perRepo).To(BeEmpty())
		})

		It("returns zero activity for pre-supplied repos when no jobs have logs", func() {
			perRepo, err := dataFactory.GetPerRepoActivity(
				context.Background(),
				ListOptions{
					Namespace: "test-namespace",
					Renovator: "test-renovator",
					Repos:     []viewmodel.GitRepoInfo{{Name: "test-repo-a"}, {Name: "test-repo-b"}},
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(perRepo).To(BeEmpty())
		})

		It("aggregates PRActivityForRenovator from per-repo activity", func() {
			perRepo, err := dataFactory.GetPerRepoActivity(
				context.Background(),
				ListOptions{Namespace: "test-namespace", Renovator: "test-renovator"},
			)
			Expect(err).NotTo(HaveOccurred())

			summary, err := dataFactory.GetPRActivityForRenovator(
				context.Background(),
				ListOptions{Namespace: "test-namespace", Renovator: "test-renovator"},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(summary.WarnCount).To(BeZero())
			Expect(summary.ErrorCount).To(BeZero())

			for _, entry := range perRepo {
				summary.Open -= entry.OpenPRs
				summary.NeedsApproval -= entry.NeedsApproval
				summary.Unchanged -= entry.Unchanged
				summary.WarnCount -= entry.WarnCount
				summary.ErrorCount -= entry.ErrorCount
			}

			Expect(summary.Open).To(BeZero())
			Expect(summary.WarnCount).To(BeZero())
			Expect(summary.ErrorCount).To(BeZero())
		})
	})

	Describe("isJobFinished", func() {
		It("returns false for a fresh job", func() {
			job := &batchv1.Job{Status: batchv1.JobStatus{}}
			Expect(isJobFinished(job)).To(BeFalse())
		})

		It("returns true when CompletionTime is set", func() {
			now := metav1.Now()
			job := &batchv1.Job{Status: batchv1.JobStatus{CompletionTime: &now}}
			Expect(isJobFinished(job)).To(BeTrue())
		})

		It("returns true when JobComplete condition is True", func() {
			job := &batchv1.Job{Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
				},
			}}
			Expect(isJobFinished(job)).To(BeTrue())
		})

		It("returns true when JobFailed condition is True", func() {
			job := &batchv1.Job{Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
				},
			}}
			Expect(isJobFinished(job)).To(BeTrue())
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
					Name:      "repo-a-older",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "test-renovator",
						renovatev1beta1.LabelGitRepo:   "repo-a",
					},
					Status: batchv1.JobStatus{CompletionTime: &older},
				},
				&batchv1.Job{
					Name:      "repo-a-newer",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "test-renovator",
						renovatev1beta1.LabelGitRepo:   "repo-a",
					},
					Status: batchv1.JobStatus{CompletionTime: &newer},
				},
				&batchv1.Job{
					Name:      "repo-b-only",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "test-renovator",
						renovatev1beta1.LabelGitRepo:   "repo-b",
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
				Name:      "long-repo-job",
				Namespace: "test-namespace",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "test-renovator",
					renovatev1beta1.LabelGitRepo:   normalized,
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
				Name:      "cached-job",
				Namespace: "test-namespace",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "test-renovator",
					renovatev1beta1.LabelGitRepo:   "test-repo-a",
				},
				Status: batchv1.JobStatus{CompletionTime: &now},
			})).To(Succeed())

			ctx := context.Background()
			opts := ListOptions{Namespace: "test-namespace", Renovator: "test-renovator"}

			first, err := dataFactory.GetPRActivityForRenovator(ctx, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(context.Background(), &batchv1.Job{
				Name:      "cached-job",
				Namespace: "test-namespace",
			})).To(Succeed())

			second, err := dataFactory.GetPRActivityForRenovator(ctx, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(second).To(Equal(first))
		})

		It("pre-supplied repos bypass cache and do not pollute it", func() {
			now := metav1.NewTime(time.Now().Add(-1 * time.Minute))
			Expect(fakeClient.Create(context.Background(), &batchv1.Job{
				Name:      "cache-isolation-job",
				Namespace: "test-namespace",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "test-renovator",
					renovatev1beta1.LabelGitRepo:   "test-repo-a",
				},
				Status: batchv1.JobStatus{CompletionTime: &now},
			})).To(Succeed())

			ctx := context.Background()
			baseOpts := ListOptions{Namespace: "test-namespace", Renovator: "test-renovator"}

			first, err := dataFactory.GetPerRepoActivity(ctx, baseOpts)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(context.Background(), &batchv1.Job{
				Name:      "cache-isolation-job",
				Namespace: "test-namespace",
			})).To(Succeed())

			repos := []viewmodel.GitRepoInfo{{Name: "test-repo-a"}}
			withRepos, err := dataFactory.GetPerRepoActivity(
				ctx,
				ListOptions{Namespace: "test-namespace", Renovator: "test-renovator", Repos: repos},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(withRepos).To(BeEmpty())

			second, err := dataFactory.GetPerRepoActivity(ctx, baseOpts)
			Expect(err).NotTo(HaveOccurred())
			Expect(second).To(Equal(first))
		})

		It("different namespace and renovator combinations have isolated caches", func() {
			now := metav1.NewTime(time.Now().Add(-1 * time.Minute))
			Expect(fakeClient.Create(context.Background(), &batchv1.Job{
				Name:      "isolated-job",
				Namespace: "test-namespace",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "test-renovator",
					renovatev1beta1.LabelGitRepo:   "test-repo-a",
				},
				Status: batchv1.JobStatus{CompletionTime: &now},
			})).To(Succeed())

			ctx := context.Background()
			opts1 := ListOptions{Namespace: "test-namespace", Renovator: "test-renovator"}

			first, err := dataFactory.GetPerRepoActivity(ctx, opts1)
			Expect(err).NotTo(HaveOccurred())

			opts2 := ListOptions{Namespace: "other-namespace", Renovator: "other-renovator"}
			other, err := dataFactory.GetPerRepoActivity(ctx, opts2)
			Expect(err).NotTo(HaveOccurred())
			Expect(other).To(BeEmpty())

			second, err := dataFactory.GetPerRepoActivity(ctx, opts1)
			Expect(err).NotTo(HaveOccurred())
			Expect(second).To(Equal(first))
		})
	})
})

// mockAuthProvider is a thin wrapper around the generated testify mock that
// pre-configures the access-control behavior we need across most tests:
//   - GetUserRepos returns the provided repo map
//   - IsUserRepo returns false for anything not explicitly mapped
//   - perIsUserRepo overrides the IsUserRepo result for specific repos (used
//     to test the single-repo fallback path in IsUserRepo)
type mockAuthProvider struct {
	*mocks.AuthProvider
	name          string
	userRepos     map[string]bool
	perIsUserRepo map[string]bool
	getErr        error
}

func newMockAuthProvider(name string) *mockAuthProvider {
	m := &mockAuthProvider{
		AuthProvider:  &mocks.AuthProvider{},
		name:          name,
		userRepos:     map[string]bool{},
		perIsUserRepo: map[string]bool{},
	}
	m.setupDefaults()

	return m
}

func (m *mockAuthProvider) setupDefaults() {
	// Simple, fixed returns for methods that don't affect access control.
	_ = m.AuthProvider.On("Type").Return("mock")
	_ = m.AuthProvider.On("Name").Return(m.name)
	_ = m.AuthProvider.On("DisplayName").Return(m.name)
	_ = m.AuthProvider.On("IconURL").Return("")
	_ = m.AuthProvider.On("LoginURL", mock.Anything, mock.Anything).Return("")

	// Access-control methods read from the wrapper's fields, so we set up
	// the expectations once and they dynamically reflect field changes.
	_ = m.AuthProvider.On("GetUserRepos", mock.Anything, mock.Anything).
		Return(func(_ context.Context, _ *http.Client) (map[string]bool, error) {
			return m.userRepos, m.getErr
		})
	_ = m.AuthProvider.On("IsUserRepo", mock.Anything, mock.Anything, mock.Anything).
		Return(func(_ context.Context, _ *http.Client, fullName string) (bool, error) {
			if v, ok := m.perIsUserRepo[fullName]; ok {
				return v, nil
			}

			return false, nil
		})
}

func (m *mockAuthProvider) setUserRepos(repos map[string]bool) {
	m.userRepos = repos
}

func (m *mockAuthProvider) setGetErr(err error) {
	m.getErr = err
}

func (m *mockAuthProvider) setPerIsUserRepo(repos map[string]bool) {
	m.perIsUserRepo = repos
}

var _ = Describe("DataFactory access filtering", func() {
	const (
		provName   = "mock-prov"
		renovatorA = "renovator-a-uid"
		renovatorB = "renovator-b-uid"
	)

	var (
		fakeClient    client.Client
		fakeClientset *kubernetesfake.Clientset
		authManager   *auth.Manager
		provider      *mockAuthProvider
		dataFactory   *DataFactory
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(batchv1.AddToScheme(scheme)).To(Succeed())

		// Two Renovators: renovator-a (accessible to user) and renovator-b (not).
		// Two GitRepos: one per Renovator.
		objects := []runtime.Object{
			&renovatev1beta1.Renovator{
				Name:      "renovator-a",
				Namespace: "test-namespace",
				UID:       types.UID(renovatorA),
				Labels:    map[string]string{renovatev1beta1.LabelAuthProvider: provName},
			},
			&renovatev1beta1.Renovator{
				Name:      "renovator-b",
				Namespace: "test-namespace",
				UID:       types.UID(renovatorB),
				Labels:    map[string]string{renovatev1beta1.LabelAuthProvider: provName},
			},
			&renovatev1beta1.GitRepo{
				Name:      "repo-a",
				Namespace: "test-namespace",
				Labels:    map[string]string{renovatev1beta1.LabelRenovator: renovatorA},
				Spec:      renovatev1beta1.GitRepoSpec{Name: "org/repo-a"},
			},
			&renovatev1beta1.GitRepo{
				Name:      "repo-b",
				Namespace: "test-namespace",
				Labels:    map[string]string{renovatev1beta1.LabelRenovator: renovatorB},
				Spec:      renovatev1beta1.GitRepoSpec{Name: "org/repo-b"},
			},
		}

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
		fakeClientset = kubernetesfake.NewClientset()

		authManager = auth.NewManager(false)
		authManager.SetIntended(true)

		provider = newMockAuthProvider(provName)
		provider.setUserRepos(map[string]bool{"org/repo-a": true, "org/repo-b": false})
		authManager.Register(provider)

		dataFactory = NewDataFactory(fakeClient, fakeClientset, authManager, nil)
	})

	// ctxWithSession returns a request context carrying a mock API session.
	// This is the same mechanism the real auth middleware uses for Bearer-token auth.
	// We also load the scs session into the context because the auth HTTP client
	// reads the session via the scs session manager on every request.
	ctxWithSession := func() context.Context {
		ctx := auth.SetAPISessionData(context.Background(), auth.SessionData{
			Subject:     "user-1",
			Provider:    provName,
			AccessToken: "test-token",
		})

		// Load scs session so the session manager can read/write without panicking.
		// The token is empty; the API session in context takes precedence.
		loaded, err := authManager.SessionManager().Load(ctx, "")
		Expect(err).NotTo(HaveOccurred())

		return loaded
	}

	Describe("GetGitRepos with auth enabled", func() {
		It("returns repos from all Renovators (Renovator-level authz)", func() {
			// GetGitRepos applies the Renovator-level authorization filter.
			// Both Renovators carry the AuthProvider label matching the session,
			// so both pass. The per-repo access filter is applied separately
			// by the handler via ApplyAccessFilter.
			repos, err := dataFactory.GetGitRepos(ctxWithSession())
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveLen(2))
		})

		It("excludes repos from Renovators with a different AuthProvider label", func() {
			// Replace renovator-b's label so it doesn't match the session provider.
			// Renovator-level authz should then exclude repo-b. Re-fetch first
			// to get the current resource version (other tests may have read it).
			var b renovatev1beta1.Renovator
			Expect(fakeClient.Get(context.Background(),
				client.ObjectKey{Namespace: "test-namespace", Name: "renovator-b"}, &b)).To(Succeed())
			b.Labels = map[string]string{renovatev1beta1.LabelAuthProvider: "other-prov"}
			Expect(fakeClient.Update(context.Background(), &b)).To(Succeed())

			repos, err := dataFactory.GetGitRepos(ctxWithSession())
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveLen(1))
			Expect(repos[0].FullName).To(Equal("org/repo-a"))
		})

		It("returns all repos when auth manager is nil", func() {
			df := NewDataFactory(fakeClient, fakeClientset, nil, nil)
			repos, err := df.GetGitRepos(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveLen(2))
		})

		It("rejects requests without a session", func() {
			// Simulate a request that passed through the auth middleware but
			// has no session cookie and no API session token.
			ctx, err := authManager.SessionManager().Load(context.Background(), "")
			Expect(err).NotTo(HaveOccurred())

			repos, err := dataFactory.GetGitRepos(ctx)
			Expect(err).To(HaveOccurred())
			Expect(repos).To(BeNil())
		})

		It("rejects requests when auth is intended but not yet ready", func() {
			// Manager with no providers but auth intended -> should fail closed
			empty := auth.NewManager(false)
			empty.SetIntended(true)
			df := NewDataFactory(fakeClient, fakeClientset, empty, nil)

			_, err := df.GetGitRepos(ctxWithSession())
			Expect(err).To(Equal(errAuthNotReady))
		})
	})

	Describe("ApplyAccessFilter", func() {
		It("returns the input unchanged when auth is disabled", func() {
			df := NewDataFactory(fakeClient, fakeClientset, nil, nil)
			repos := []viewmodel.GitRepoInfo{
				{FullName: "org/repo-a"},
				{FullName: "org/repo-b"},
			}
			filtered := df.ApplyAccessFilter(context.Background(), repos)
			Expect(filtered).To(HaveLen(2))
		})

		It("filters repos based on the user's accessible repo list", func() {
			repos := []viewmodel.GitRepoInfo{
				{FullName: "org/repo-a"},
				{FullName: "org/repo-b"},
				{FullName: "org/repo-c"},
			}
			filtered := dataFactory.ApplyAccessFilter(ctxWithSession(), repos)
			// Only org/repo-a is in the user's accessible set
			Expect(filtered).To(HaveLen(1))
			Expect(filtered[0].FullName).To(Equal("org/repo-a"))
		})

		It("returns empty list when provider returns an error (fail closed)", func() {
			provider.setGetErr(errors.New("upstream failure"))

			repos := []viewmodel.GitRepoInfo{{FullName: "org/repo-a"}}
			filtered := dataFactory.ApplyAccessFilter(ctxWithSession(), repos)
			Expect(filtered).To(BeEmpty())
		})

		It("returns empty list when session is missing", func() {
			ctx, err := authManager.SessionManager().Load(context.Background(), "")
			Expect(err).NotTo(HaveOccurred())

			repos := []viewmodel.GitRepoInfo{{FullName: "org/repo-a"}}
			filtered := dataFactory.ApplyAccessFilter(ctx, repos)
			Expect(filtered).To(BeEmpty())
		})
	})

	Describe("IsUserRepo", func() {
		It("returns true when the repo is in the user's accessible list", func() {
			Expect(dataFactory.IsUserRepo(ctxWithSession(), "org/repo-a")).To(BeTrue())
		})

		It("returns false when the repo is not in the user's accessible list", func() {
			Expect(dataFactory.IsUserRepo(ctxWithSession(), "org/repo-b")).To(BeFalse())
		})

		It("returns true when auth is disabled", func() {
			df := NewDataFactory(fakeClient, fakeClientset, nil, nil)
			Expect(df.IsUserRepo(context.Background(), "any/repo")).To(BeTrue())
		})

		It("returns false when session is missing", func() {
			ctx, err := authManager.SessionManager().Load(context.Background(), "")
			Expect(err).NotTo(HaveOccurred())

			Expect(dataFactory.IsUserRepo(ctx, "org/repo-a")).To(BeFalse())
		})

		It("falls back to provider.IsUserRepo for repos not in the cached list", func() {
			// Populate the cache with one repo so the cache is non-empty (avoiding
			// the empty-cache fast path), but exclude the repo we're about to
			// check. This forces the single-repo fallback to provider.IsUserRepo.
			provider.setUserRepos(map[string]bool{"org/repo-a": true})
			provider.setPerIsUserRepo(map[string]bool{"org/extra": true})

			ctx := auth.SetAPISessionData(context.Background(), auth.SessionData{
				Subject:     "user-2",
				Provider:    provName,
				AccessToken: "test-token-2",
			})
			loaded, err := authManager.SessionManager().Load(ctx, "")
			Expect(err).NotTo(HaveOccurred())

			Expect(dataFactory.IsUserRepo(loaded, "org/extra")).To(BeTrue())
		})
	})
})

var _ = Describe("readJobLogStream", func() {
	DescribeTable(
		"truncation detection",
		func(input string, tailLines int64, expectedContent string, expectedTruncated bool) {
			content, truncated, err := readJobLogStream(strings.NewReader(input), tailLines)
			Expect(err).NotTo(HaveOccurred())
			Expect(content).To(Equal(expectedContent))
			Expect(truncated).To(Equal(expectedTruncated))
		},
		Entry("tailLines=0 returns full content", "line1\nline2\nline3\n", int64(0), "line1\nline2\nline3\n", false),
		Entry("fewer lines than tail", "line1\nline2\n", int64(5), "line1\nline2\n", false),
		Entry("exactly tailLines lines", "line1\nline2\nline3\n", int64(3), "line1\nline2\nline3\n", false),
		Entry("more lines than tail (truncated)", "line1\nline2\nline3\nline4\n", int64(2), "line1\nline2", true),
		Entry("empty content", "", int64(5), "", false),
		Entry("single line with tail", "line1\n", int64(1), "line1\n", false),
		Entry("single line without tail", "line1\n", int64(0), "line1\n", false),
		Entry("content without trailing newline", "line1\nline2\nline3", int64(2), "line1\nline2", true),
	)

	It("returns an error when reading fails", func() {
		failingReader := &failingReader{}
		_, _, err := readJobLogStream(failingReader, int64(5))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("read error"))
	})
})

type failingReader struct{}

func (f *failingReader) Read([]byte) (int, error) {
	return 0, errors.New("read error")
}
