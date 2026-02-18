package frontend

import (
	"context"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Server", func() {
	var (
		client client.Client
		server *Server
		config ServerConfig
	)

	BeforeEach(func() {
		client = fake.NewClientBuilder().Build()
		config = DefaultServerConfig()
		server = NewServer(config, client)
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

			customServer := NewServer(customConfig, client)

			Expect(customServer).NotTo(BeNil())
		})
	})

	Describe("Start and Stop", func() {
		It("should start and stop the server", func() {
			err := server.Start()
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(100 * time.Millisecond)

			resp, err := http.Get("http://" + config.Addr)
			if err == nil {
				defer resp.Body.Close()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = server.Stop(ctx)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("DefaultServerConfig", func() {
		It("should return default server configuration", func() {
			defaultConfig := DefaultServerConfig()

			Expect(defaultConfig.Addr).To(Equal(":8080"))
			Expect(defaultConfig.ReadTimeout).To(Equal(10 * time.Second))
			Expect(defaultConfig.WriteTimeout).To(Equal(30 * time.Second))
			Expect(defaultConfig.IdleTimeout).To(Equal(120 * time.Second))
		})
	})
})
