package telemetry

import (
	"context"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	prombridge "go.opentelemetry.io/contrib/bridges/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

const (
	defaultExportInterval = 15 * time.Second
)

// noopShutdown is returned when OTLP export is disabled, so callers can
// always invoke the shutdown function without a nil check.
func noopShutdown(context.Context) error { return nil }

// SetupPrometheusBridge configures an OpenTelemetry MeterProvider that bridges
// Prometheus metrics to OTLP/gRPC export. This allows all existing Prometheus
// metrics (including controller-runtime metrics) to be exported via OTLP
// without any code changes to the metrics themselves.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is not set the function returns a no-op
// shutdown and no error: OTLP export is disabled. Otherwise it returns the
// MeterProvider's Shutdown function, which the caller should invoke on
// shutdown to flush pending metrics.
func SetupPrometheusBridge(
	ctx context.Context, gatherer prometheus.Gatherer, version string,
) (func(context.Context) error, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return noopShutdown, nil
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			attribute.String("service.name", "renovate-operator"),
			attribute.String("service.version", version),
		),
	)
	if err != nil {
		return noopShutdown, err
	}

	exporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return noopShutdown, err
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
