package frontend

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/thegeeklab/renovate-operator/internal/logstore"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var frontendLog = logf.Log.WithName("frontend")

// ServerConfig holds configuration for the HTTP server.
type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// Default timeouts for the HTTP server.
const (
	DefaultReadTimeout  = 10 * time.Second
	DefaultWriteTimeout = 30 * time.Second
	DefaultIdleTimeout  = 120 * time.Second
)

// DefaultServerConfig returns default server configuration.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:         ":8080",
		ReadTimeout:  DefaultReadTimeout,
		WriteTimeout: DefaultWriteTimeout,
		IdleTimeout:  DefaultIdleTimeout,
	}
}

// Server manages the HTTP server.
type Server struct {
	config           ServerConfig
	router           *mux.Router
	server           *http.Server
	apiHandler       *APIHandler
	dashboardHandler *WebHandler
}

// NewServer creates a new HTTP server instance.
func NewServer(config ServerConfig, client client.Client, logManager *logstore.Manager, broker *SSEBroker) *Server {
	apiHandler := NewAPIHandler(client, logManager)

	dashboardHandler := NewWebHandler(client, logManager, broker)

	router := mux.NewRouter()

	s := &Server{
		config:           config,
		router:           router,
		apiHandler:       apiHandler,
		dashboardHandler: dashboardHandler,
	}

	apiHandler.RegisterRoutes(router)
	dashboardHandler.RegisterRoutes(router)

	s.server = &http.Server{
		Addr:         config.Addr,
		Handler:      s.router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return s
}

// Start runs the HTTP server in a separate goroutine.
func (s *Server) Start() error {
	go func() {
		frontendLog.Info("Starting HTTP server", "address", s.config.Addr)

		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			frontendLog.Error(err, "HTTP server error")
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	frontendLog.Info("Shutting down HTTP server")

	return s.server.Shutdown(ctx)
}
