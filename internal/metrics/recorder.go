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

type Recorder interface {
	RecordGitRepoRun(namespace, renovator, runner, gitrepo, status string)
	SetRunFailed(namespace, renovator, runner, gitrepo string, failed bool)
	SetLastRunTimestamp(namespace, renovator, runner, gitrepo string, timestamp float64)
	SetDependencyIssues(namespace, renovator, runner, gitrepo string, hasIssues bool)
	SetApprovalsNeeded(namespace, renovator, runner, gitrepo string, count int)
	DeleteGitRepo(namespace, renovator, runner, gitrepo string)
	RecordRunnerReconcileDuration(duration time.Duration, result string)
	Gatherer() prometheus.Gatherer
}

type recorder struct {
	gitrepoRuns             *prometheus.CounterVec
	gitrepoRunFailed        *prometheus.GaugeVec
	gitrepoLastRun          *prometheus.GaugeVec
	gitrepoDependencyIssues *prometheus.GaugeVec
	gitrepoApprovalsNeeded  *prometheus.GaugeVec
	runnerReconcileDur      *prometheus.HistogramVec
	seriesDropped           *prometheus.CounterVec
	guard                   *CardinalityGuard
	gatherer                prometheus.Gatherer
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
		runnerReconcileDur, seriesDropped,
	)

	r := &recorder{
		gitrepoRuns:             gitrepoRuns,
		gitrepoRunFailed:        gitrepoRunFailed,
		gitrepoLastRun:          gitrepoLastRun,
		gitrepoDependencyIssues: gitrepoDependencyIssues,
		gitrepoApprovalsNeeded:  gitrepoApprovalsNeeded,
		runnerReconcileDur:      runnerReconcileDur,
		seriesDropped:           seriesDropped,
		guard:                   guard,
		gatherer:                gatherer,
	}

	return r
}

//nolint:ireturn
func (r *recorder) Gatherer() prometheus.Gatherer {
	return r.gatherer
}
