package receiver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var receiverLog = logf.Log.WithName("receiver-server")

const (
	DefaultReadTimeout  = 10 * time.Second
	DefaultWriteTimeout = 30 * time.Second
	DefaultIdleTimeout  = 120 * time.Second
)

type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:         ":8083",
		ReadTimeout:  DefaultReadTimeout,
		WriteTimeout: DefaultWriteTimeout,
		IdleTimeout:  DefaultIdleTimeout,
	}
}

type Server struct {
	config  ServerConfig
	router  *mux.Router
	server  *http.Server
	handler *Handler
}

func NewServer(config ServerConfig, k8sClient client.Client) *Server {
	s := &Server{
		config: config,
		router: mux.NewRouter(),
	}

	s.handler = NewHandler(k8sClient)
	s.handler.RegisterRoutes(s.router)

	s.server = &http.Server{
		Addr:         config.Addr,
		Handler:      s.router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return s
}

func (s *Server) Start() error {
	go func() {
		receiverLog.Info("Starting Event Receiver server", "address", s.config.Addr)

		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			receiverLog.Error(err, "HTTP server error")
		}
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	receiverLog.Info("Shutting down Event Receiver server")

	return s.server.Shutdown(ctx)
}
