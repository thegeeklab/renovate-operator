package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
	StatusUnknown    = "unknown"
	StatusDispatched = "dispatched"

	KindRunner    = "runner"
	KindDiscovery = "discovery"

	histogramBucketStart  = 0.01
	histogramBucketFactor = 2
	histogramBucketCount  = 15
)

//nolint:interfacebloat
type Recorder interface {
	// --- GitRepo-scoped (namespace, renovator, runner, gitrepo) ---
	RecordGitRepoRun(namespace, renovator, runner, gitrepo, status string)
	SetRunFailed(namespace, renovator, runner, gitrepo string, failed bool)
	SetLastRunTimestamp(namespace, renovator, runner, gitrepo string, timestamp float64)
	SetLastRunDuration(namespace, renovator, runner, gitrepo string, seconds float64)
	SetDependencyIssues(namespace, renovator, runner, gitrepo string, hasIssues bool)
	SetApprovalsNeeded(namespace, renovator, runner, gitrepo string, count int)
	SetDependenciesTotal(namespace, renovator, runner, gitrepo string, count int)
	SetDependenciesOutdated(namespace, renovator, runner, gitrepo string, count int)
	SetDependencyUpdates(namespace, renovator, runner, gitrepo, updateType string, count int)
	SetVulnerabilityFixesAvailable(namespace, renovator, runner, gitrepo string, count int)
	SetBranchResults(namespace, renovator, runner, gitrepo, result string, count int)
	SetLogWarnCount(namespace, renovator, runner, gitrepo string, count int)
	SetLogErrorCount(namespace, renovator, runner, gitrepo string, count int)
	DeleteGitRepo(namespace, renovator, runner, gitrepo string)

	// --- Runner-scoped (namespace, renovator, runner) ---
	RecordRunnerJob(namespace, renovator, runner, status string)
	RecordRunnerJobFailure(namespace, renovator, runner, reason string)
	RecordRunnerJobDuration(namespace, renovator, runner, status string, seconds float64)
	SetRunnerQueueDepth(namespace, renovator, runner string, count int)
	SetRunnerRunning(namespace, renovator, runner string, count int)
	RecordRunnerScheduleRun(namespace, renovator, runner, result string)
	SetRunnerScheduleNextRun(namespace, renovator, runner string, timestamp float64)
	DeleteRunner(namespace, renovator, runner string)

	// --- Discovery-scoped (namespace, renovator, discovery) ---
	RecordDiscoveryJob(namespace, renovator, discovery, status string)
	RecordDiscoveryJobFailure(namespace, renovator, discovery, reason string)
	SetDiscoveryRepositories(namespace, renovator, discovery string, count int)
	DeleteDiscovery(namespace, renovator, discovery string)

	// --- Webhook (provider, result) ---
	RecordWebhookRequest(provider, result string)
	RecordWebhookSignatureFailure(provider string)
	RecordWebhookAuthFailure(provider, errorType string)
	RecordWebhookPayloadDecodeFailure(provider string)

	// --- Secret resolution ---
	RecordSecretResolutionError(errorType string)

	// --- Reconciler instrumentation ---
	RecordReconcileDuration(kind, result string, seconds float64)

	// --- Accessor ---
	Gatherer() prometheus.Gatherer
}

type recorder struct {
	// GitRepo-scoped (4 labels)
	gitrepoRuns                 *prometheus.CounterVec
	gitrepoRunFailed            *prometheus.GaugeVec
	gitrepoLastRun              *prometheus.GaugeVec
	gitrepoLastRunDur           *prometheus.GaugeVec
	gitrepoDependencyIssues     *prometheus.GaugeVec
	gitrepoApprovalsNeeded      *prometheus.GaugeVec
	gitrepoDependenciesTotal    *prometheus.GaugeVec
	gitrepoDependenciesOutdated *prometheus.GaugeVec
	gitrepoDependencyUpdates    *prometheus.GaugeVec
	gitrepoVulnerabilityFixes   *prometheus.GaugeVec
	gitrepoBranchResults        *prometheus.GaugeVec
	gitrepoLogWarnings          *prometheus.GaugeVec
	gitrepoLogErrors            *prometheus.GaugeVec

	// Runner-scoped (3 labels)
	runnerJobs            *prometheus.CounterVec
	runnerJobFailures     *prometheus.CounterVec
	runnerJobDuration     *prometheus.HistogramVec
	runnerQueueDepth      *prometheus.GaugeVec
	runnerRunning         *prometheus.GaugeVec
	runnerScheduleRuns    *prometheus.CounterVec
	runnerScheduleNextRun *prometheus.GaugeVec

	// Discovery-scoped (3 labels)
	discoveryJobs        *prometheus.CounterVec
	discoveryJobFailures *prometheus.CounterVec
	discoveryRepoCount   *prometheus.GaugeVec

	// Webhook (provider, result)
	webhookRequests              *prometheus.CounterVec
	webhookSignatureFailures     *prometheus.CounterVec
	webhookAuthFailures          *prometheus.CounterVec
	webhookPayloadDecodeFailures *prometheus.CounterVec

	// Secret resolution
	secretResolutionErrors *prometheus.CounterVec

	// Reconciler instrumentation
	reconcileDur *prometheus.HistogramVec

	// Internal
	seriesDropped *prometheus.CounterVec
	guard         *CardinalityGuard
	gatherer      prometheus.Gatherer
}

var _ Recorder = (*recorder)(nil)

//nolint:ireturn,maintidx
func New(reg prometheus.Registerer, gatherer prometheus.Gatherer, cardinalityCap int) Recorder {
	guard := NewCardinalityGuard(cardinalityCap)

	gitrepoRuns := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_gitrepo_runs_total",
			Help: "Total number of GitRepo runs by status.",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo", "status"},
	)

	gitrepoRunFailed := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_gitrepo_run_failed",
			Help: "Whether the last run for a GitRepo failed (1=failed, 0=not failed).",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo"},
	)

	gitrepoLastRun := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_gitrepo_last_run_timestamp_seconds",
			Help: "Unix timestamp of the last run for a GitRepo.",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo"},
	)

	gitrepoLastRunDur := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_gitrepo_last_run_duration_seconds",
			Help: "Wall-clock duration of the most recent GitRepo run in seconds.",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo"},
	)

	gitrepoDependencyIssues := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_gitrepo_dependency_issues",
			Help: "Whether the last Renovate run produced WARN or ERROR log entries (1=issues found, 0=clean).",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo"},
	)

	gitrepoApprovalsNeeded := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_gitrepo_approvals_needed",
			Help: "Number of dependency updates awaiting approval.",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo"},
	)

	gitrepoDependenciesTotal := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_gitrepo_dependencies",
			Help: "Total number of managed dependencies in the last Renovate run.",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo"},
	)

	gitrepoDependenciesOutdated := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_gitrepo_dependencies_outdated",
			Help: "Number of dependencies with available updates.",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo"},
	)

	gitrepoDependencyUpdates := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_gitrepo_dependency_updates",
			Help: "Number of pending dependency updates by type (major, minor, patch, pin, digest).",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo", "update_type"},
	)

	gitrepoVulnerabilityFixes := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_gitrepo_vulnerability_fixes_available",
			Help: "Number of dependencies with available vulnerability fixes.",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo"},
	)

	gitrepoBranchResults := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_gitrepo_branch_results",
			Help: "Number of branches by result type (created, updated, already-existed, not-scheduled, etc.).",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo", "result"},
	)

	gitrepoLogWarnings := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_gitrepo_log_warnings",
			Help: "Number of WARN-level log entries observed in the last Renovate run.",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo"},
	)

	gitrepoLogErrors := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_gitrepo_log_errors",
			Help: "Number of ERROR-level log entries observed in the last Renovate run.",
		},
		[]string{"namespace", "renovator", "runner", "gitrepo"},
	)

	runnerJobs := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_runner_jobs_total",
			Help: "Total number of GitRepo jobs by status (dispatched, succeeded, failed).",
		},
		[]string{"namespace", "renovator", "runner", "status"},
	)

	runnerJobDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "renovate_operator_runner_job_duration_seconds",
			Help:    "Wall-clock duration of runner Kubernetes Jobs.",
			Buckets: prometheus.ExponentialBuckets(histogramBucketStart, histogramBucketFactor, histogramBucketCount),
		},
		[]string{"namespace", "renovator", "runner", "status"},
	)

	runnerJobFailures := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_runner_job_failures_total",
			Help: "Total number of failed runner Kubernetes Jobs by failure reason.",
		},
		[]string{"namespace", "renovator", "runner", "reason"},
	)

	runnerQueueDepth := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_runner_queue_depth",
			Help: "Number of GitRepo resources waiting to be processed (queue depth).",
		},
		[]string{"namespace", "renovator", "runner"},
	)

	runnerRunning := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_runner_running",
			Help: "Number of GitRepo resources currently being processed (in-flight count).",
		},
		[]string{"namespace", "renovator", "runner"},
	)

	runnerScheduleRuns := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_runner_schedule_runs_total",
			Help: "Total number of cron schedule firings executed by result.",
		},
		[]string{"namespace", "renovator", "runner", "result"},
	)

	runnerScheduleNextRun := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_runner_schedule_next_run_timestamp_seconds",
			Help: "Unix timestamp of the next planned scheduled run.",
		},
		[]string{"namespace", "renovator", "runner"},
	)

	discoveryRepoCount := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "renovate_operator_discovery_repositories",
			Help: "Number of repositories seen by the last discovery run.",
		},
		[]string{"namespace", "renovator", "discovery"},
	)

	discoveryJobs := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_discovery_jobs_total",
			Help: "Total number of discovery Kubernetes Jobs by status.",
		},
		[]string{"namespace", "renovator", "discovery", "status"},
	)

	discoveryJobFailures := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_discovery_job_failures_total",
			Help: "Total number of failed discovery Kubernetes Jobs by failure reason.",
		},
		[]string{"namespace", "renovator", "discovery", "reason"},
	)

	webhookRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_webhook_requests_total",
			Help: "Total number of webhook requests by provider and result.",
		},
		[]string{"provider", "result"},
	)

	webhookSignatureFailures := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_webhook_signature_verification_failures_total",
			Help: "Total number of webhook HMAC signature verification failures.",
		},
		[]string{"provider"},
	)

	webhookAuthFailures := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_webhook_auth_failures_total",
			Help: "Total number of webhook authentication failures by error type.",
		},
		[]string{"provider", "error_type"},
	)

	webhookPayloadDecodeFailures := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_webhook_payload_decode_failures_total",
			Help: "Total number of webhook payloads that failed to decode.",
		},
		[]string{"provider"},
	)

	secretResolutionErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_secret_resolution_errors_total",
			Help: "Total number of Kubernetes Secret resolution errors by error type.",
		},
		[]string{"error_type"},
	)

	reconcileDur := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "renovate_operator_reconcile_duration_seconds",
			Help:    "Duration of a single reconcile loop tick.",
			Buckets: prometheus.ExponentialBuckets(histogramBucketStart, histogramBucketFactor, histogramBucketCount),
		},
		[]string{"kind", "result"},
	)

	seriesDropped := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_metrics_series_dropped_total",
			Help: "Total number of metric series dropped due to cardinality cap.",
		},
		[]string{"reason"},
	)

	reg.MustRegister(
		gitrepoRuns, gitrepoRunFailed, gitrepoLastRun, gitrepoLastRunDur,
		gitrepoDependencyIssues, gitrepoApprovalsNeeded,
		gitrepoDependenciesTotal, gitrepoDependenciesOutdated,
		gitrepoDependencyUpdates, gitrepoVulnerabilityFixes,
		gitrepoBranchResults, gitrepoLogWarnings, gitrepoLogErrors,
		runnerJobs, runnerJobFailures, runnerJobDuration,
		runnerQueueDepth, runnerRunning,
		runnerScheduleRuns, runnerScheduleNextRun,
		discoveryJobs, discoveryJobFailures, discoveryRepoCount,
		webhookRequests, webhookSignatureFailures,
		webhookAuthFailures, webhookPayloadDecodeFailures,
		secretResolutionErrors,
		reconcileDur, seriesDropped,
	)

	r := &recorder{
		gitrepoRuns:                  gitrepoRuns,
		gitrepoRunFailed:             gitrepoRunFailed,
		gitrepoLastRun:               gitrepoLastRun,
		gitrepoLastRunDur:            gitrepoLastRunDur,
		gitrepoDependencyIssues:      gitrepoDependencyIssues,
		gitrepoApprovalsNeeded:       gitrepoApprovalsNeeded,
		gitrepoDependenciesTotal:     gitrepoDependenciesTotal,
		gitrepoDependenciesOutdated:  gitrepoDependenciesOutdated,
		gitrepoDependencyUpdates:     gitrepoDependencyUpdates,
		gitrepoVulnerabilityFixes:    gitrepoVulnerabilityFixes,
		gitrepoBranchResults:         gitrepoBranchResults,
		gitrepoLogWarnings:           gitrepoLogWarnings,
		gitrepoLogErrors:             gitrepoLogErrors,
		runnerJobs:                   runnerJobs,
		runnerJobFailures:            runnerJobFailures,
		runnerJobDuration:            runnerJobDuration,
		runnerQueueDepth:             runnerQueueDepth,
		runnerRunning:                runnerRunning,
		runnerScheduleRuns:           runnerScheduleRuns,
		runnerScheduleNextRun:        runnerScheduleNextRun,
		discoveryJobs:                discoveryJobs,
		discoveryJobFailures:         discoveryJobFailures,
		discoveryRepoCount:           discoveryRepoCount,
		webhookRequests:              webhookRequests,
		webhookSignatureFailures:     webhookSignatureFailures,
		webhookAuthFailures:          webhookAuthFailures,
		webhookPayloadDecodeFailures: webhookPayloadDecodeFailures,
		secretResolutionErrors:       secretResolutionErrors,
		reconcileDur:                 reconcileDur,
		seriesDropped:                seriesDropped,
		guard:                        guard,
		gatherer:                     gatherer,
	}

	return r
}

//nolint:ireturn
func (r *recorder) Gatherer() prometheus.Gatherer {
	return r.gatherer
}
