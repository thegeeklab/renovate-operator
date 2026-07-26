package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

func (r *recorder) RecordGitRepoRun(namespace, renovator, runner, gitrepo, status string) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoRuns.WithLabelValues(namespace, renovator, runner, gitrepo, status).Inc()
}

func (r *recorder) SetRunFailed(namespace, renovator, runner, gitrepo string, failed bool) {
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

func (r *recorder) SetLastRunTimestamp(namespace, renovator, runner, gitrepo string, timestamp float64) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoLastRun.WithLabelValues(namespace, renovator, runner, gitrepo).Set(timestamp)
}

func (r *recorder) SetLastRunDuration(namespace, renovator, runner, gitrepo string, seconds float64) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoLastRunDur.WithLabelValues(namespace, renovator, runner, gitrepo).Set(seconds)
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

func (r *recorder) SetDependenciesTotal(namespace, renovator, runner, gitrepo string, count int) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoDependenciesTotal.WithLabelValues(namespace, renovator, runner, gitrepo).Set(float64(count))
}

func (r *recorder) SetDependenciesOutdated(namespace, renovator, runner, gitrepo string, count int) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoDependenciesOutdated.WithLabelValues(namespace, renovator, runner, gitrepo).Set(float64(count))
}

func (r *recorder) SetDependencyUpdates(namespace, renovator, runner, gitrepo, updateType string, count int) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoDependencyUpdates.WithLabelValues(namespace, renovator, runner, gitrepo, updateType).Set(float64(count))
}

func (r *recorder) SetVulnerabilityFixesAvailable(namespace, renovator, runner, gitrepo string, count int) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoVulnerabilityFixes.WithLabelValues(namespace, renovator, runner, gitrepo).Set(float64(count))
}

func (r *recorder) SetBranchResults(namespace, renovator, runner, gitrepo, result string, count int) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoBranchResults.WithLabelValues(namespace, renovator, runner, gitrepo, result).Set(float64(count))
}

func (r *recorder) SetLogWarnCount(namespace, renovator, runner, gitrepo string, count int) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoLogWarnings.WithLabelValues(namespace, renovator, runner, gitrepo).Set(float64(count))
}

func (r *recorder) SetLogErrorCount(namespace, renovator, runner, gitrepo string, count int) {
	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	if !r.guard.Allow(key) {
		r.seriesDropped.WithLabelValues("cardinality_cap").Inc()

		return
	}

	r.gitrepoLogErrors.WithLabelValues(namespace, renovator, runner, gitrepo).Set(float64(count))
}

func (r *recorder) DeleteGitRepo(namespace, renovator, runner, gitrepo string) {
	r.gitrepoRuns.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace, "renovator": renovator, "runner": runner, "gitrepo": gitrepo,
	})

	r.gitrepoRunFailed.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoLastRun.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoLastRunDur.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoDependencyIssues.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoApprovalsNeeded.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoDependenciesTotal.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoDependenciesOutdated.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoVulnerabilityFixes.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoLogWarnings.DeleteLabelValues(namespace, renovator, runner, gitrepo)
	r.gitrepoLogErrors.DeleteLabelValues(namespace, renovator, runner, gitrepo)

	r.gitrepoDependencyUpdates.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace, "renovator": renovator, "runner": runner, "gitrepo": gitrepo,
	})
	r.gitrepoBranchResults.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace, "renovator": renovator, "runner": runner, "gitrepo": gitrepo,
	})

	key := gitrepoKey(namespace, renovator, runner, gitrepo)
	r.guard.Remove(key)
}

func (r *recorder) RecordReconcileDuration(kind, result string, seconds float64) {
	r.reconcileDur.WithLabelValues(kind, result).Observe(seconds)
}

func (r *recorder) RecordRunnerJob(namespace, renovator, runner, status string) {
	r.runnerJobs.WithLabelValues(namespace, renovator, runner, status).Inc()
}

func (r *recorder) RecordRunnerJobDuration(namespace, renovator, runner, status string, seconds float64) {
	r.runnerJobDuration.WithLabelValues(namespace, renovator, runner, status).Observe(seconds)
}

func (r *recorder) SetRunnerQueueDepth(namespace, renovator, runner string, count int) {
	r.runnerQueueDepth.WithLabelValues(namespace, renovator, runner).Set(float64(count))
}

func (r *recorder) SetRunnerRunning(namespace, renovator, runner string, count int) {
	r.runnerRunning.WithLabelValues(namespace, renovator, runner).Set(float64(count))
}

func (r *recorder) RecordRunnerScheduleRun(namespace, renovator, runner, result string) {
	r.runnerScheduleRuns.WithLabelValues(namespace, renovator, runner, result).Inc()
}

func (r *recorder) SetRunnerScheduleNextRun(namespace, renovator, runner string, timestamp float64) {
	r.runnerScheduleNextRun.WithLabelValues(namespace, renovator, runner).Set(timestamp)
}

func (r *recorder) RecordDiscoveryJob(namespace, renovator, discovery, status string) {
	r.discoveryJobs.WithLabelValues(namespace, renovator, discovery, status).Inc()
}

func (r *recorder) SetDiscoveryRepositories(namespace, renovator, discovery string, count int) {
	r.discoveryRepoCount.WithLabelValues(namespace, renovator, discovery).Set(float64(count))
}

func (r *recorder) RecordWebhookRequest(provider, result string) {
	r.webhookRequests.WithLabelValues(provider, result).Inc()
}

func (r *recorder) RecordWebhookSignatureFailure(provider string) {
	r.webhookSignatureFailures.WithLabelValues(provider).Inc()
}

func (r *recorder) RecordWebhookAuthFailure(provider, errorType string) {
	r.webhookAuthFailures.WithLabelValues(provider, errorType).Inc()
}

func (r *recorder) RecordWebhookPayloadDecodeFailure(provider string) {
	r.webhookPayloadDecodeFailures.WithLabelValues(provider).Inc()
}

func (r *recorder) RecordSecretResolutionError(errorType string) {
	r.secretResolutionErrors.WithLabelValues(errorType).Inc()
}

func (r *recorder) DeleteRunner(namespace, renovator, runner string) {
	r.runnerJobs.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace, "renovator": renovator, "runner": runner,
	})
	r.runnerJobDuration.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace, "renovator": renovator, "runner": runner,
	})
	r.runnerQueueDepth.DeleteLabelValues(namespace, renovator, runner)
	r.runnerRunning.DeleteLabelValues(namespace, renovator, runner)
	r.runnerScheduleRuns.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace, "renovator": renovator, "runner": runner,
	})
	r.runnerScheduleNextRun.DeleteLabelValues(namespace, renovator, runner)
}

func (r *recorder) DeleteDiscovery(namespace, renovator, discovery string) {
	r.discoveryJobs.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace, "renovator": renovator, "discovery": discovery,
	})
	r.discoveryRepoCount.DeleteLabelValues(namespace, renovator, discovery)
}

func gitrepoKey(namespace, renovator, runner, gitrepo string) string {
	return fmt.Sprintf("%s/%s/%s/%s", namespace, renovator, runner, gitrepo)
}
