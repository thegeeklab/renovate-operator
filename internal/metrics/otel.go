package metrics

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	api "go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

type otelMirror struct {
	gitrepoRuns api.Int64Counter
}

func newOTelMirror() *otelMirror {
	meter := otel.Meter("github.com/thegeeklab/renovate-operator")

	counter, _ := meter.Int64Counter(
		"renovate_operator.gitrepo.runs",
		api.WithDescription("Total number of GitRepo runs by status."),
		api.WithUnit("{execution}"),
	)

	return &otelMirror{
		gitrepoRuns: counter,
	}
}

func (o *otelMirror) recordGitRepoRun(namespace, renovator, runner, gitrepo, status string) {
	if o.gitrepoRuns == nil {
		return
	}

	attrs := attribute.NewSet(
		semconv.K8SNamespaceName(namespace),
		attribute.String("renovate.operator.renovator", renovator),
		attribute.String("renovate.operator.runner", runner),
		attribute.String("renovate.operator.gitrepo", gitrepo),
		attribute.String("cicd.pipeline.result", status),
	)

	o.gitrepoRuns.Add(context.Background(), 1, api.WithAttributeSet(attrs))
}
