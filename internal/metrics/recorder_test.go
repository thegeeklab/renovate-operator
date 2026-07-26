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

		It("should record reconcile duration", func() {
			rec.RecordReconcileDuration(KindRunner, "success", 0.1)
			rec.RecordReconcileDuration(KindDiscovery, "error", 0.2)

			count := testutil.CollectAndCount(recImpl.reconcileDur)
			Expect(count).To(BeNumerically(">", 0))
		})

		It("should set last run duration gauge", func() {
			rec.SetLastRunDuration("default", "test-renovator", "test-runner", "test-repo", 42.5)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_last_run_duration_seconds Wall-clock duration of the most recent GitRepo run in seconds.
				# TYPE renovate_operator_gitrepo_last_run_duration_seconds gauge
				renovate_operator_gitrepo_last_run_duration_seconds{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner"} 42.5
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoLastRunDur, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
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

		It("should set dependencies total gauge", func() {
			rec.SetDependenciesTotal("default", "test-renovator", "test-runner", "test-repo", 42)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_dependencies Total number of managed dependencies in the last Renovate run.
				# TYPE renovate_operator_gitrepo_dependencies gauge
				renovate_operator_gitrepo_dependencies{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner"} 42
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoDependenciesTotal, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set dependencies outdated gauge", func() {
			rec.SetDependenciesOutdated("default", "test-renovator", "test-runner", "test-repo", 10)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_dependencies_outdated Number of dependencies with available updates.
				# TYPE renovate_operator_gitrepo_dependencies_outdated gauge
				renovate_operator_gitrepo_dependencies_outdated{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner"} 10
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoDependenciesOutdated, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set dependency updates gauge by type", func() {
			rec.SetDependencyUpdates("default", "test-renovator", "test-runner", "test-repo", "major", 3)
			rec.SetDependencyUpdates("default", "test-renovator", "test-runner", "test-repo", "minor", 7)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_dependency_updates Number of pending dependency updates by type (major, minor, patch, pin, digest).
				# TYPE renovate_operator_gitrepo_dependency_updates gauge
				renovate_operator_gitrepo_dependency_updates{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner",update_type="major"} 3
				renovate_operator_gitrepo_dependency_updates{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner",update_type="minor"} 7
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoDependencyUpdates, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set vulnerability fixes available gauge", func() {
			rec.SetVulnerabilityFixesAvailable("default", "test-renovator", "test-runner", "test-repo", 2)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_vulnerability_fixes_available Number of dependencies with available vulnerability fixes.
				# TYPE renovate_operator_gitrepo_vulnerability_fixes_available gauge
				renovate_operator_gitrepo_vulnerability_fixes_available{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner"} 2
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoVulnerabilityFixes, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set branch results gauge by result type", func() {
			rec.SetBranchResults("default", "test-renovator", "test-runner", "test-repo", "created", 5)
			rec.SetBranchResults("default", "test-renovator", "test-runner", "test-repo", "updated", 3)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_branch_results Number of branches by result type (created, updated, already-existed, not-scheduled, etc.).
				# TYPE renovate_operator_gitrepo_branch_results gauge
				renovate_operator_gitrepo_branch_results{gitrepo="test-repo",namespace="default",renovator="test-renovator",result="created",runner="test-runner"} 5
				renovate_operator_gitrepo_branch_results{gitrepo="test-repo",namespace="default",renovator="test-renovator",result="updated",runner="test-runner"} 3
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoBranchResults, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set log warnings count gauge", func() {
			rec.SetLogWarnCount("default", "test-renovator", "test-runner", "test-repo", 4)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_log_warnings Number of WARN-level log entries observed in the last Renovate run.
				# TYPE renovate_operator_gitrepo_log_warnings gauge
				renovate_operator_gitrepo_log_warnings{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner"} 4
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoLogWarnings, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set log errors count gauge", func() {
			rec.SetLogErrorCount("default", "test-renovator", "test-runner", "test-repo", 2)

			//nolint:lll
			expected := `
				# HELP renovate_operator_gitrepo_log_errors Number of ERROR-level log entries observed in the last Renovate run.
				# TYPE renovate_operator_gitrepo_log_errors gauge
				renovate_operator_gitrepo_log_errors{gitrepo="test-repo",namespace="default",renovator="test-renovator",runner="test-runner"} 2
			`

			err := testutil.CollectAndCompare(recImpl.gitrepoLogErrors, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete gitrepo metrics", func() {
			rec.RecordGitRepoRun("default", "test-renovator", "test-runner", "test-repo", StatusSucceeded)
			rec.SetRunFailed("default", "test-renovator", "test-runner", "test-repo", false)
			rec.SetLastRunTimestamp("default", "test-renovator", "test-runner", "test-repo", 1234567890)
			rec.SetLastRunDuration("default", "test-renovator", "test-runner", "test-repo", 42.5)
			rec.SetDependencyIssues("default", "test-renovator", "test-runner", "test-repo", false)
			rec.SetApprovalsNeeded("default", "test-renovator", "test-runner", "test-repo", 3)
			rec.SetDependenciesTotal("default", "test-renovator", "test-runner", "test-repo", 50)
			rec.SetDependenciesOutdated("default", "test-renovator", "test-runner", "test-repo", 10)
			rec.SetDependencyUpdates("default", "test-renovator", "test-runner", "test-repo", "major", 2)
			rec.SetVulnerabilityFixesAvailable("default", "test-renovator", "test-runner", "test-repo", 1)
			rec.SetBranchResults("default", "test-renovator", "test-runner", "test-repo", "created", 5)
			rec.SetLogWarnCount("default", "test-renovator", "test-runner", "test-repo", 3)
			rec.SetLogErrorCount("default", "test-renovator", "test-runner", "test-repo", 1)

			rec.DeleteGitRepo("default", "test-renovator", "test-runner", "test-repo")

			Expect(testutil.CollectAndCount(recImpl.gitrepoRuns)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoRunFailed)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoLastRun)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoLastRunDur)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoDependencyIssues)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoApprovalsNeeded)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoDependenciesTotal)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoDependenciesOutdated)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoDependencyUpdates)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoVulnerabilityFixes)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoBranchResults)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoLogWarnings)).To(Equal(0))
			Expect(testutil.CollectAndCount(recImpl.gitrepoLogErrors)).To(Equal(0))
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

		It("should record runner job counter", func() {
			rec.RecordRunnerJob("default", "test-renovator", "test-runner", StatusSucceeded)
			rec.RecordRunnerJob("default", "test-renovator", "test-runner", StatusFailed)
			rec.RecordRunnerJob("default", "test-renovator", "test-runner", StatusDispatched)

			//nolint:lll
			expected := `
				# HELP renovate_operator_runner_jobs_total Total number of GitRepo jobs by status (dispatched, succeeded, failed).
				# TYPE renovate_operator_runner_jobs_total counter
				renovate_operator_runner_jobs_total{namespace="default",renovator="test-renovator",runner="test-runner",status="dispatched"} 1
				renovate_operator_runner_jobs_total{namespace="default",renovator="test-renovator",runner="test-runner",status="failed"} 1
				renovate_operator_runner_jobs_total{namespace="default",renovator="test-renovator",runner="test-runner",status="succeeded"} 1
			`

			err := testutil.CollectAndCompare(recImpl.runnerJobs, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should record runner job duration", func() {
			rec.RecordRunnerJobDuration("default", "test-renovator", "test-runner", StatusSucceeded, 45.0)

			count := testutil.CollectAndCount(recImpl.runnerJobDuration)
			Expect(count).To(Equal(1))
		})

		It("should set runner queue depth and running gauges", func() {
			rec.SetRunnerQueueDepth("default", "test-renovator", "test-runner", 12)
			rec.SetRunnerRunning("default", "test-renovator", "test-runner", 3)

			depth := testutil.ToFloat64(
				recImpl.runnerQueueDepth.WithLabelValues("default", "test-renovator", "test-runner"),
			)
			Expect(depth).To(Equal(12.0))

			running := testutil.ToFloat64(
				recImpl.runnerRunning.WithLabelValues("default", "test-renovator", "test-runner"),
			)
			Expect(running).To(Equal(3.0))
		})

		It("should record schedule run counter and set next run timestamp", func() {
			rec.RecordRunnerScheduleRun("default", "test-renovator", "test-runner", "success")
			rec.RecordRunnerScheduleRun("default", "test-renovator", "test-runner", "error")
			rec.SetRunnerScheduleNextRun("default", "test-renovator", "test-runner", 1714000000)

			//nolint:lll
			expected := `
				# HELP renovate_operator_runner_schedule_runs_total Total number of cron schedule firings executed by result.
				# TYPE renovate_operator_runner_schedule_runs_total counter
				renovate_operator_runner_schedule_runs_total{namespace="default",renovator="test-renovator",result="error",runner="test-runner"} 1
				renovate_operator_runner_schedule_runs_total{namespace="default",renovator="test-renovator",result="success",runner="test-runner"} 1
			`

			err := testutil.CollectAndCompare(recImpl.runnerScheduleRuns, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())

			nextRun := testutil.ToFloat64(
				recImpl.runnerScheduleNextRun.WithLabelValues("default", "test-renovator", "test-runner"),
			)
			Expect(nextRun).To(Equal(1714000000.0))
		})

		It("should record discovery repo count", func() {
			rec.SetDiscoveryRepositories("default", "test-renovator", "test-discovery", 25)

			repos := testutil.ToFloat64(
				recImpl.discoveryRepoCount.WithLabelValues("default", "test-renovator", "test-discovery"),
			)
			Expect(repos).To(Equal(25.0))
		})

		It("should record webhook metrics", func() {
			rec.RecordWebhookRequest("github", "accepted")
			rec.RecordWebhookRequest("github", "rejected")
			rec.RecordWebhookSignatureFailure("github")

			expected := `
				# HELP renovate_operator_webhook_requests_total Total number of webhook requests by provider and result.
				# TYPE renovate_operator_webhook_requests_total counter
				renovate_operator_webhook_requests_total{provider="github",result="accepted"} 1
				renovate_operator_webhook_requests_total{provider="github",result="rejected"} 1
			`

			err := testutil.CollectAndCompare(recImpl.webhookRequests, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())

			sigFail := testutil.ToFloat64(
				recImpl.webhookSignatureFailures.WithLabelValues("github"),
			)
			Expect(sigFail).To(Equal(1.0))
		})

		It("should record webhook auth failure", func() {
			rec.RecordWebhookAuthFailure("github", "no_matching_job")
			rec.RecordWebhookAuthFailure("gitlab", "secret_error")

			expected := `
				# HELP renovate_operator_webhook_auth_failures_total Total number of webhook authentication failures by error type.
				# TYPE renovate_operator_webhook_auth_failures_total counter
				renovate_operator_webhook_auth_failures_total{error_type="secret_error",provider="gitlab"} 1
				renovate_operator_webhook_auth_failures_total{error_type="no_matching_job",provider="github"} 1
			`

			err := testutil.CollectAndCompare(recImpl.webhookAuthFailures, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should record webhook payload decode failure", func() {
			rec.RecordWebhookPayloadDecodeFailure("github")
			rec.RecordWebhookPayloadDecodeFailure("github")
			rec.RecordWebhookPayloadDecodeFailure("gitlab")

			//nolint:lll
			expected := `
				# HELP renovate_operator_webhook_payload_decode_failures_total Total number of webhook payloads that failed to decode.
				# TYPE renovate_operator_webhook_payload_decode_failures_total counter
				renovate_operator_webhook_payload_decode_failures_total{provider="github"} 2
				renovate_operator_webhook_payload_decode_failures_total{provider="gitlab"} 1
			`

			err := testutil.CollectAndCompare(recImpl.webhookPayloadDecodeFailures, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should record secret resolution error", func() {
			rec.RecordSecretResolutionError("not_found")
			rec.RecordSecretResolutionError("key_missing")

			//nolint:lll
			expected := `
				# HELP renovate_operator_secret_resolution_errors_total Total number of Kubernetes Secret resolution errors by error type.
				# TYPE renovate_operator_secret_resolution_errors_total counter
				renovate_operator_secret_resolution_errors_total{error_type="key_missing"} 1
				renovate_operator_secret_resolution_errors_total{error_type="not_found"} 1
			`

			err := testutil.CollectAndCompare(recImpl.secretResolutionErrors, strings.NewReader(expected))
			Expect(err).NotTo(HaveOccurred())
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
