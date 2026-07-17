package metrics

import (
	"fmt"
	"time"

	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
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
	r.otel.recordGitRepoRun(namespace, renovator, runner, gitrepo, status)
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

func (r *recorder) DeleteGitRepo(namespace, renovator, runner, gitrepo string) {
	for _, status := range []string{StatusSucceeded, StatusFailed, StatusUnknown} {
		r.gitrepoRuns.DeleteLabelValues(namespace, renovator, runner, gitrepo, status)
	}

	r.gitrepoRunFailed.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoLastRun.DeleteLabelValues(namespace, renovator, runner, gitrepo)

	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	r.guard.Remove(key)
}

func (r *recorder) RecordRunnerReconcileDuration(duration time.Duration, result string) {
	r.runnerReconcileDur.WithLabelValues(result).Observe(duration.Seconds())
}

func gitrepoKey(namespace, renovator, runner, gitrepo string) string {
	return fmt.Sprintf("%s/%s/%s/%s", namespace, renovator, runner, gitrepo)
}

func SanitizeGitRepoLabel(name string) (string, error) {
	return k8s.SanitizeLabel(name)
}
