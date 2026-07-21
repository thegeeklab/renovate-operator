package metrics

import (
	"fmt"
	"time"
)

func (r *recorder) RecordGitRepoRun(
	namespace, renovator, runner, gitrepo, status string,
) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoRuns.WithLabelValues(namespace, renovator, runner, gitrepo, status).Inc()
}

func (r *recorder) SetRunFailed(
	namespace, renovator, runner, gitrepo string, failed bool,
) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	val := 0.0
	if failed {
		val = 1.0
	}

	r.gitrepoRunFailed.WithLabelValues(namespace, renovator, runner, gitrepo).Set(val)
}

func (r *recorder) SetLastRunTimestamp(
	namespace, renovator, runner, gitrepo string, timestamp float64,
) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoLastRun.WithLabelValues(namespace, renovator, runner, gitrepo).Set(timestamp)
}

func (r *recorder) SetDependencyIssues(namespace, renovator, runner, gitrepo string, hasIssues bool) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	val := 0.0
	if hasIssues {
		val = 1.0
	}

	r.gitrepoDependencyIssues.WithLabelValues(namespace, renovator, runner, gitrepo).Set(val)
}

func (r *recorder) SetApprovalsNeeded(namespace, renovator, runner, gitrepo string, count int) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoApprovalsNeeded.WithLabelValues(namespace, renovator, runner, gitrepo).Set(float64(count))
}

func (r *recorder) DeleteGitRepo(namespace, renovator, runner, gitrepo string) {
	for _, status := range []string{StatusSucceeded, StatusFailed, StatusUnknown} {
		r.gitrepoRuns.DeleteLabelValues(namespace, renovator, runner, gitrepo, status)
	}

	r.gitrepoRunFailed.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoLastRun.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoDependencyIssues.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoApprovalsNeeded.DeleteLabelValues(namespace, renovator, runner, gitrepo)

	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	r.guard.Remove(key)
}

func (r *recorder) RecordRunnerReconcileDuration(duration time.Duration, result string) {
	r.runnerReconcileDur.WithLabelValues(result).Observe(duration.Seconds())
}

func gitrepoKey(namespace, renovator, runner, gitrepo string) string {
	return fmt.Sprintf("%s/%s/%s/%s", namespace, renovator, runner, gitrepo)
}
