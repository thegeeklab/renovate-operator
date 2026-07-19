package telemetry

import (
	"context"
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

// SetupPrometheusBridge configures an OpenTelemetry MeterProvider that bridges
// Prometheus metrics to OTLP/gRPC export. This allows all existing Prometheus
// metrics (including controller-runtime metrics) to be exported via OTLP
// without any code changes to the metrics themselves.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is not set the function returns (nil, nil):
// OTLP export is disabled and no closer is needed. Otherwise it returns the
// MeterProvider's Shutdown function, which the caller should invoke on shutdown
// to flush pending metrics.
func SetupPrometheusBridge(
	ctx context.Context, gatherer prometheus.Gatherer, version string,
) (func(context.Context) error, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return nil, nil //nolint:nilnil // disabled: no closer needed
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

	return provider.Shutdown, nil
}
