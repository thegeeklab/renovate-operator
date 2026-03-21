package frontend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/thegeeklab/renovate-operator/internal/logstore"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	frontendLog = logf.Log.WithName("frontend")

	errAssetManifest    = errors.New("could not read asset manifest")
	errMainEntryMissing = errors.New("main entry point not found in asset manifest")
)

const (
	DefaultReadTimeout  = 10 * time.Second
	DefaultWriteTimeout = 30 * time.Second
	DefaultIdleTimeout  = 120 * time.Second
)

// ServerConfig holds configuration for the HTTP server.
type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	DevMode      bool
}

// assetManifest is used internally to unmarshal the bundler's build output.
type assetManifest map[string]struct {
	File string   `json:"file"`
	CSS  []string `json:"css"`
}

// DefaultServerConfig returns default server configuration.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:         ":8080",
		ReadTimeout:  DefaultReadTimeout,
		WriteTimeout: DefaultWriteTimeout,
		IdleTimeout:  DefaultIdleTimeout,
		DevMode:      false,
	}
}

// Server manages the HTTP server.
type Server struct {
	config           ServerConfig
	assets           FrontendAssets
	router           *mux.Router
	server           *http.Server
	apiHandler       *APIHandler
	dashboardHandler *WebHandler
}

// NewServer creates a new HTTP server instance.
func NewServer(config ServerConfig, client client.Client, logManager *logstore.Manager, broker *SSEBroker) *Server {
	s := &Server{
		config: config,
		router: mux.NewRouter(),
	}

	if err := s.loadFrontendAssets(); err != nil {
		frontendLog.Error(err, "Failed to load frontend assets (UI might be broken)")
	} else {
		frontendLog.Info("Frontend assets loaded", "devMode", s.config.DevMode)
	}

	s.apiHandler = NewAPIHandler(client, logManager)
	s.dashboardHandler = NewWebHandler(client, logManager, broker, s.assets)

	s.apiHandler.RegisterRoutes(s.router)
	s.dashboardHandler.RegisterRoutes(s.router)

	if !s.config.DevMode {
		staticDir := "internal/frontend/static/dist"
		fs := http.FileServer(http.Dir(staticDir))
		s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	}

	s.server = &http.Server{
		Addr:         config.Addr,
		Handler:      s.router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return s
}

// loadFrontendAssets populates the server's FrontendAssets struct based on the configuration.
func (s *Server) loadFrontendAssets() error {
	if s.config.DevMode {
		s.assets.Scripts = []string{
			"http://localhost:5173/@vite/client",
			"http://localhost:5173/internal/frontend/static/main.js",
		}

		s.assets.Styles = []string{
			"http://localhost:5173/internal/frontend/static/style.css",
		}

		return nil
	}

	manifestPath := "internal/frontend/static/dist/.vite/manifest.json"

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("%w: %w", errAssetManifest, err)
	}

	var manifest assetManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}

	entry, ok := manifest["internal/frontend/static/main.js"]
	if !ok {
		return errMainEntryMissing
	}

	s.assets.Scripts = []string{"/static/" + entry.File}

	if len(entry.CSS) > 0 {
		s.assets.Styles = []string{"/static/" + entry.CSS[0]}
	} else {
		s.assets.Styles = []string{}
	}

	return nil
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
