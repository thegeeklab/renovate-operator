package frontend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Server", func() {
	var (
		fakeClient client.Client
		server     *Server
		config     ServerConfig
		broker     *SSEBroker
	)

	BeforeEach(func() {
		fakeClient = fake.NewClientBuilder().Build()
		broker = NewSSEBroker()
		config = DefaultServerConfig()
		server = NewServer(config, fakeClient, nil, broker, nil, nil)
	})

	Describe("NewServer", func() {
		It("should create a new server with default config", func() {
			Expect(server).NotTo(BeNil())
		})

		It("should create a new server with custom config", func() {
			customConfig := ServerConfig{
				Addr:         ":9090",
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 15 * time.Second,
				IdleTimeout:  60 * time.Second,
			}

			customServer := NewServer(customConfig, fakeClient, nil, broker, nil, nil)

			Expect(customServer).NotTo(BeNil())
		})
	})

	Describe("Start and Shutdown", func() {
		It("should start and gracefully shut down the server when context is cancelled", func() {
			config.Addr = "127.0.0.1:18082"
			server = NewServer(config, fakeClient, nil, broker, nil, nil)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			errChan := make(chan error, 1)

			go func() {
				errChan <- server.Start(ctx)
			}()

			time.Sleep(100 * time.Millisecond)

			resp, err := http.Get("http://" + config.Addr + "/")
			if err == nil {
				defer resp.Body.Close()
			}

			Expect(err).NotTo(HaveOccurred())

			cancel()

			var startErr error
			Eventually(errChan, 3*time.Second).Should(Receive(&startErr))
			Expect(startErr).NotTo(HaveOccurred())
		})
	})

	Describe("Readiness", func() {
		It("should not report ready before the listener is bound", func() {
			config.Addr = "127.0.0.1:18083"
			server = NewServer(config, fakeClient, nil, broker, nil, nil)

			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

			Expect(server.StartedChecker()(req)).To(HaveOccurred())
		})

		It("should report ready once the listener is bound", func() {
			config.Addr = "127.0.0.1:18084"
			server = NewServer(config, fakeClient, nil, broker, nil, nil)

			check := server.StartedChecker()
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			errChan := make(chan error, 1)

			go func() {
				errChan <- server.Start(ctx)
			}()

			Eventually(func() error { return check(req) }).Should(Succeed())

			cancel()

			var startErr error
			Eventually(errChan, 3*time.Second).Should(Receive(&startErr))
			Expect(startErr).NotTo(HaveOccurred())
		})
	})

	Describe("DefaultServerConfig", func() {
		It("should return default server configuration", func() {
			defaultConfig := DefaultServerConfig()

			Expect(defaultConfig.Addr).To(Equal(":8082"))
			Expect(defaultConfig.ReadTimeout).To(Equal(10 * time.Second))
			Expect(defaultConfig.WriteTimeout).To(Equal(30 * time.Second))
			Expect(defaultConfig.IdleTimeout).To(Equal(120 * time.Second))
		})
	})
})
