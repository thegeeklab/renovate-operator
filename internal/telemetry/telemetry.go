package telemetry

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	prombridge "go.opentelemetry.io/contrib/bridges/prometheus"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

const (
	defaultExportInterval = 15 * time.Second
)

var ErrOTLPEndpointNotSet = errors.New("OTEL_EXPORTER_OTLP_ENDPOINT is not set")

// SetupPrometheusBridge creates an OpenTelemetry MeterProvider that bridges
// Prometheus metrics to OTLP export. This allows all existing Prometheus metrics
// (including controller-runtime metrics) to be exported via OTLP without any
// code changes to the metrics themselves.
//
// Returns ErrOTLPEndpointNotSet if OTEL_EXPORTER_OTLP_ENDPOINT is not set (OTLP disabled).
func SetupPrometheusBridge(
	ctx context.Context, gatherer prometheus.Gatherer, version string,
) (*sdkmetric.MeterProvider, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return nil, ErrOTLPEndpointNotSet
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("renovate-operator"),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, err
	}

	exporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	promBridge := prombridge.NewMetricProducer(
		prombridge.WithGatherer(gatherer),
	)

	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(defaultExportInterval),
		sdkmetric.WithProducer(promBridge),
	)

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)

	return provider, nil
}
