package webui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServerConfig holds configuration for the HTTP server.
type ServerConfig struct {
	Addr         string        // Address to listen on (e.g., ":8080")
	ReadTimeout  time.Duration // Read timeout for HTTP server
	WriteTimeout time.Duration // Write timeout for HTTP server
	IdleTimeout  time.Duration // Idle timeout for HTTP server
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
	config     ServerConfig
	router     *mux.Router
	server     *http.Server
	apiHandler *APIHandler
}

// NewServer creates a new HTTP server instance.
func NewServer(config ServerConfig, client client.Client) *Server {
	apiHandler := NewAPIHandler(client)
	router := mux.NewRouter()

	s := &Server{
		config:     config,
		router:     router,
		apiHandler: apiHandler,
	}

	// Register API routes
	apiHandler.RegisterRoutes(router)

	// Create HTTP server
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
		fmt.Printf("Starting HTTP server on %s\n", s.config.Addr)

		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	fmt.Println("Shutting down HTTP server...")

	return s.server.Shutdown(ctx)
}
