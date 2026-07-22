package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusUnknown   = "unknown"

	histogramBucketStart  = 0.01
	histogramBucketFactor = 2
	histogramBucketCount  = 15
)

//nolint:interfacebloat
type Recorder interface {
	RecordGitRepoRun(namespace, renovator, runner, gitrepo, status string)
	SetRunFailed(namespace, renovator, runner, gitrepo string, failed bool)
	SetLastRunTimestamp(namespace, renovator, runner, gitrepo string, timestamp float64)
	SetDependencyIssues(namespace, renovator, runner, gitrepo string, hasIssues bool)
	SetApprovalsNeeded(namespace, renovator, runner, gitrepo string, count int)
	SetDependenciesTotal(namespace, renovator, runner, gitrepo string, count int)
	SetDependenciesOutdated(namespace, renovator, runner, gitrepo string, count int)
	SetDependencyUpdates(namespace, renovator, runner, gitrepo, updateType string, count int)
	SetVulnerabilityFixesAvailable(namespace, renovator, runner, gitrepo string, count int)
	SetBranchResults(namespace, renovator, runner, gitrepo, result string, count int)
	DeleteGitRepo(namespace, renovator, runner, gitrepo string)
	RecordRunnerReconcileDuration(duration time.Duration, result string)
	Gatherer() prometheus.Gatherer
}

type recorder struct {
	gitrepoRuns                 *prometheus.CounterVec
	gitrepoRunFailed            *prometheus.GaugeVec
	gitrepoLastRun              *prometheus.GaugeVec
	gitrepoDependencyIssues     *prometheus.GaugeVec
	gitrepoApprovalsNeeded      *prometheus.GaugeVec
	gitrepoDependenciesTotal    *prometheus.GaugeVec
	gitrepoDependenciesOutdated *prometheus.GaugeVec
	gitrepoDependencyUpdates    *prometheus.GaugeVec
	gitrepoVulnerabilityFixes   *prometheus.GaugeVec
	gitrepoBranchResults        *prometheus.GaugeVec
	runnerReconcileDur          *prometheus.HistogramVec
	seriesDropped               *prometheus.CounterVec
	guard                       *CardinalityGuard
	gatherer                    prometheus.Gatherer
}

var _ Recorder = (*recorder)(nil)

//nolint:ireturn
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

	runnerReconcileDur := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "renovate_operator_runner_reconcile_duration_seconds",
			Help:    "Duration of runner reconcile operations.",
			Buckets: prometheus.ExponentialBuckets(histogramBucketStart, histogramBucketFactor, histogramBucketCount),
		},
		[]string{"result"},
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

	seriesDropped := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "renovate_operator_metrics_series_dropped_total",
			Help: "Total number of metric series dropped due to cardinality cap.",
		},
		[]string{"reason"},
	)

	reg.MustRegister(
		gitrepoRuns, gitrepoRunFailed, gitrepoLastRun,
		gitrepoDependencyIssues, gitrepoApprovalsNeeded,
		gitrepoDependenciesTotal, gitrepoDependenciesOutdated,
		gitrepoDependencyUpdates, gitrepoVulnerabilityFixes,
		gitrepoBranchResults,
		runnerReconcileDur, seriesDropped,
	)

	r := &recorder{
		gitrepoRuns:                 gitrepoRuns,
		gitrepoRunFailed:            gitrepoRunFailed,
		gitrepoLastRun:              gitrepoLastRun,
		gitrepoDependencyIssues:     gitrepoDependencyIssues,
		gitrepoApprovalsNeeded:      gitrepoApprovalsNeeded,
		gitrepoDependenciesTotal:    gitrepoDependenciesTotal,
		gitrepoDependenciesOutdated: gitrepoDependenciesOutdated,
		gitrepoDependencyUpdates:    gitrepoDependencyUpdates,
		gitrepoVulnerabilityFixes:   gitrepoVulnerabilityFixes,
		gitrepoBranchResults:        gitrepoBranchResults,
		runnerReconcileDur:          runnerReconcileDur,
		seriesDropped:               seriesDropped,
		guard:                       guard,
		gatherer:                    gatherer,
	}

	return r
}

//nolint:ireturn
func (r *recorder) Gatherer() prometheus.Gatherer {
	return r.gatherer
}
