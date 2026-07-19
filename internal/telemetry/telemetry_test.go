package telemetry

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/prometheus/client_golang/prometheus"
)

var _ = Describe("Telemetry", func() {
	BeforeEach(func() {
		// Reset environment before each test
		os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	})

	AfterEach(func() {
		// Clean up environment variables
		os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	})

	Describe("SetupPrometheusBridge", func() {
		Context("when OTEL_EXPORTER_OTLP_ENDPOINT is not set", func() {
			BeforeEach(func() {
				os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
			})

			It("should return ErrOTLPEndpointNotSet", func() {
				gatherer := prometheus.NewRegistry()
				provider, err := SetupPrometheusBridge(context.Background(), gatherer, "dev")
				Expect(err).To(MatchError(ErrOTLPEndpointNotSet))
				Expect(provider).To(BeNil())
			})
		})

		Context("when OTEL_EXPORTER_OTLP_ENDPOINT is set", func() {
			BeforeEach(func() {
				os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
			})

			It("should return a provider", func() {
				gatherer := prometheus.NewRegistry()
				provider, err := SetupPrometheusBridge(context.Background(), gatherer, "dev")
				Expect(err).NotTo(HaveOccurred())
				Expect(provider).NotTo(BeNil())

				// Cleanup
				if provider != nil {
					_ = provider.Shutdown(context.Background())
				}
			})

			It("should not panic on shutdown", func() {
				gatherer := prometheus.NewRegistry()
				provider, err := SetupPrometheusBridge(context.Background(), gatherer, "dev")
				Expect(err).NotTo(HaveOccurred())

				if provider != nil {
					Expect(func() { _ = provider.Shutdown(context.Background()) }).NotTo(Panic())
				}
			})
		})
	})
})
