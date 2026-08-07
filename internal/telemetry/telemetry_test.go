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
			It("should return a no-op closer and no error", func() {
				gatherer := prometheus.NewRegistry()
				shutdown, err := SetupPrometheusBridge(context.Background(), gatherer, "dev")
				Expect(err).NotTo(HaveOccurred())
				Expect(shutdown).NotTo(BeNil())
				Expect(shutdown(context.Background())).To(Succeed())
			})
		})

		Context("when OTEL_EXPORTER_OTLP_ENDPOINT is set", func() {
			BeforeEach(func() {
				os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
			})

			It("should return a non-nil closer and no error", func() {
				gatherer := prometheus.NewRegistry()
				shutdown, err := SetupPrometheusBridge(context.Background(), gatherer, "dev")
				Expect(err).NotTo(HaveOccurred())
				Expect(shutdown).NotTo(BeNil())

				_ = shutdown(context.Background())
			})
		})
	})
})
