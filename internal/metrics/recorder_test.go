package metrics

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var _ = Describe("Recorder", func() {
	Describe("Prometheus", func() {
		var (
			reg     *prometheus.Registry
			rec     Recorder
			recImpl *recorder
		)

		BeforeEach(func() {
			reg = prometheus.NewRegistry()
			rec = New(reg, reg, 5000)

			var ok bool

			recImpl, ok = rec.(*recorder)
			Expect(ok).To(BeTrue())
		})

		It("should record gitrepo runs", func() {
			rec.RecordGitRepoRun("default", "test-renovator", "test-runner", "test-repo", StatusSucceeded)
			rec.RecordGitRepoRun("default", "test-renovator", "test-runner", "test-repo", StatusFailed)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_runs_total Total number of GitRepo runs by status.
				# TYPE renovate_operator_gitrepo_runs_total counter
				renovate_operator_gitrepo_runs_total{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner",status="failed"} 1
				renovate_operator_gitrepo_runs_total{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner",status="succeeded"} 1
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoRuns, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set run failed gauge", func() {
			rec.SetRunFailed("default", "test-renovator", "test-runner", "test-repo", true)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_run_failed Whether the last run for a GitRepo failed (1=failed, 0=not failed).
				# TYPE renovate_operator_gitrepo_run_failed gauge
				renovate_operator_gitrepo_run_failed{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner"} 1
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoRunFailed, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set last run timestamp gauge", func() {
			ts := float64(time.Date(2026, 2, 27, 15, 0, 0, 0, time.UTC).Unix())
			rec.SetLastRunTimestamp("default", "test-renovator", "test-runner", "test-repo", ts)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_last_run_timestamp_seconds Unix timestamp of the last run for a GitRepo.
				# TYPE renovate_operator_gitrepo_last_run_timestamp_seconds gauge
				renovate_operator_gitrepo_last_run_timestamp_seconds{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner"} 1.7722044e+09
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoLastRun, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should record runner reconcile duration", func() {
			rec.RecordRunnerReconcileDuration(100*time.Millisecond, "success")
			rec.RecordRunnerReconcileDuration(200*time.Millisecond, "error")

			count := testutil.CollectAndCount(recImpl.runnerReconcileDur)
			Expect(count).To(BeNumerically(">", 0))
		})

		It("should set dependency issues gauge", func() {
			rec.SetDependencyIssues("default", "test-renovator", "test-runner", "test-repo", true)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_dependency_issues Whether the last Renovate run produced WARN or ERROR log entries (1=issues found, 0=clean).
				# TYPE renovate_operator_gitrepo_dependency_issues gauge
				renovate_operator_gitrepo_dependency_issues{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner"} 1
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoDependencyIssues, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set approvals needed gauge", func() {
			rec.SetApprovalsNeeded("default", "test-renovator", "test-runner", "test-repo", 5)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_approvals_needed Number of dependency updates awaiting approval.
				# TYPE renovate_operator_gitrepo_approvals_needed gauge
				renovate_operator_gitrepo_approvals_needed{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner"} 5
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoApprovalsNeeded, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete gitrepo metrics", func() {
			rec.RecordGitRepoRun("default", "test-renovator", "test-runner", "test-repo", StatusSucceeded)
			rec.SetRunFailed("default", "test-renovator", "test-runner", "test-repo", false)
			rec.SetLastRunTimestamp("default", "test-renovator", "test-runner", "test-repo", 1234567890)
			rec.SetDependencyIssues("default", "test-renovator", "test-runner", "test-repo", false)
			rec.SetApprovalsNeeded("default", "test-renovator", "test-runner", "test-repo", 3)

			rec.DeleteGitRepo("default", "test-renovator", "test-runner", "test-repo")

			Expect(testutil.CollectAndCount(recImpl.gitrepoRuns)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoRunFailed)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoLastRun)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoDependencyIssues)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoApprovalsNeeded)).To(Equal(0))
		})

		It("should enforce cardinality cap", func() {
			smallReg := prometheus.NewRegistry()
			smallRec := New(smallReg, smallReg, 2)
			smallImpl, ok := smallRec.(*recorder)
			Expect(ok).To(BeTrue())

			smallRec.RecordGitRepoRun("default", "test-renovator", "test-runner", "repo-1", StatusSucceeded)
			smallRec.RecordGitRepoRun("default", "test-renovator", "test-runner", "repo-2", StatusSucceeded)
			smallRec.RecordGitRepoRun("default", "test-renovator", "test-runner", "repo-3", StatusSucceeded)

			dropped := testutil.ToFloat64(smallImpl.seriesDropped.WithLabelValues("cardinality_cap"))
			Expect(dropped).To(Equal(1.0))
		})

		It("should increment counter correctly across multiple calls", func() {
			rec.RecordGitRepoRun("default", "test-renovator", "test-runner", "test-repo", StatusSucceeded)
			rec.RecordGitRepoRun("default", "test-renovator", "test-runner", "test-repo", StatusSucceeded)
			rec.RecordGitRepoRun("default", "test-renovator", "test-runner", "test-repo", StatusSucceeded)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_runs_total Total number of GitRepo runs by status.
				# TYPE renovate_operator_gitrepo_runs_total counter
				renovate_operator_gitrepo_runs_total{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner",status="succeeded"} 3
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoRuns, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update gauge values correctly", func() {
			rec.SetRunFailed("default", "test-renovator", "test-runner", "test-repo", false)

			val := testutil.ToFloat64(
				recImpl.gitrepoRunFailed.WithLabelValues("default", "test-renovator", "test-runner", "test-repo"),
			)
			Expect(val).To(Equal(0.0))

			rec.SetRunFailed("default", "test-renovator", "test-runner", "test-repo", true)

			val = testutil.ToFloat64(
				recImpl.gitrepoRunFailed.WithLabelValues("default", "test-renovator", "test-runner", "test-repo"),
			)
			Expect(val).To(Equal(1.0))

			rec.SetRunFailed("default", "test-renovator", "test-runner", "test-repo", false)

			val = testutil.ToFloat64(
				recImpl.gitrepoRunFailed.WithLabelValues("default", "test-renovator", "test-runner", "test-repo"),
			)
			Expect(val).To(Equal(0.0))
		})

		It("should handle multiple namespaces and repos independently", func() {
			rec.RecordGitRepoRun("ns-1", "renovator-1", "runner-1", "repo-1", StatusSucceeded)
			rec.RecordGitRepoRun("ns-1", "renovator-1", "runner-1", "repo-2", StatusFailed)
			rec.RecordGitRepoRun("ns-2", "renovator-2", "runner-2", "repo-1", StatusSucceeded)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_runs_total Total number of GitRepo runs by status.
				# TYPE renovate_operator_gitrepo_runs_total counter
				renovate_operator_gitrepo_runs_total{gitrepo="repo-1",namespace="ns-1",renovator="renovator-1",runner="runner-1",status="succeeded"} 1
				renovate_operator_gitrepo_runs_total{gitrepo="repo-1",namespace="ns-2",renovator="renovator-2",runner="runner-2",status="succeeded"} 1
				renovate_operator_gitrepo_runs_total{gitrepo="repo-2",namespace="ns-1",renovator="renovator-1",runner="runner-1",status="failed"} 1
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoRuns, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should only delete metrics for the specified gitrepo", func() {
			rec.RecordGitRepoRun("default", "test-renovator", "test-runner", "repo-1", StatusSucceeded)
			rec.RecordGitRepoRun("default", "test-renovator", "test-runner", "repo-2", StatusSucceeded)

			rec.DeleteGitRepo("default", "test-renovator", "test-runner", "repo-1")

			Expect(testutil.CollectAndCount(recImpl.gitrepoRuns)).To(Equal(1))

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_runs_total Total number of GitRepo runs by status.
				# TYPE renovate_operator_gitrepo_runs_total counter
				renovate_operator_gitrepo_runs_total{gitrepo="repo-2",namespace="default",renovator="test-renovator",runner="test-runner",status="succeeded"} 1
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoRuns, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("CardinalityGuard", func() {
		It("should allow keys up to the cap", func() {
			guard := NewCardinalityGuard(2)

			Expect(guard.Allow("key1")).To(BeTrue())
			Expect(guard.Allow("key2")).To(BeTrue())
			Expect(guard.Allow("key3")).To(BeFalse())
		})

		It("should allow repeated keys without counting", func() {
			guard := NewCardinalityGuard(1)

			Expect(guard.Allow("key1")).To(BeTrue())
			Expect(guard.Allow("key1")).To(BeTrue())
			Expect(guard.Allow("key2")).To(BeFalse())
		})

		It("should allow new keys after removal", func() {
			guard := NewCardinalityGuard(1)

			Expect(guard.Allow("key1")).To(BeTrue())
			guard.Remove("key1")
			Expect(guard.Allow("key2")).To(BeTrue())
		})
	})
})
