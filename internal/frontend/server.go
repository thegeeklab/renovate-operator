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
	"github.com/thegeeklab/renovate-operator/internal/auth"
	"k8s.io/client-go/kubernetes"
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
	Addr          string
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	IdleTimeout   time.Duration
	DevMode       bool
	SecureCookies bool
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
	config      ServerConfig
	assets      FrontendAssets
	router      *mux.Router
	server      *http.Server
	apiHandler  *APIHandler
	webHandler  *WebHandler
	authManager *auth.Manager
}

// NewServer creates a new HTTP server instance.
func NewServer(
	config ServerConfig,
	client client.Client,
	clientset kubernetes.Interface,
	broker *SSEBroker,
	authManager *auth.Manager,
) *Server {
	s := &Server{
		config:      config,
		router:      mux.NewRouter(),
		authManager: authManager,
	}

	if err := s.loadFrontendAssets(); err != nil {
		frontendLog.Error(err, "Failed to load frontend assets (UI might be broken)")
	} else {
		frontendLog.Info("Frontend assets loaded", "devMode", s.config.DevMode)
	}

	s.apiHandler = NewAPIHandler(client, clientset, authManager)
	s.webHandler = NewWebHandler(client, clientset, broker, s.assets, authManager)

	s.apiHandler.RegisterRoutes(s.router)
	s.webHandler.RegisterRoutes(s.router)

	if authManager != nil && authManager.IsEnabled() {
		s.registerAuthRoutes()
	}

	if !s.config.DevMode {
		staticDir := "internal/frontend/static/dist"
		fs := http.FileServer(http.Dir(staticDir))
		s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	}

	var handler http.Handler = s.router
	if authManager != nil && authManager.IsEnabled() {
		handler = auth.Middleware(authManager)(s.router)
	}

	s.server = &http.Server{
		Addr:         config.Addr,
		Handler:      handler,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return s
}

func (s *Server) registerAuthRoutes() {
	s.router.HandleFunc("/auth/login", auth.HandleLogin(s.authManager, s.config.SecureCookies)).Methods("GET")
	s.router.HandleFunc("/auth/callback", auth.HandleCallback(s.authManager, s.config.SecureCookies)).Methods("GET")
	s.router.HandleFunc("/auth/logout", auth.HandleLogout(s.authManager)).Methods("POST")
	s.router.HandleFunc("/api/v1/auth/status", auth.HandleAuthStatus(s.authManager)).Methods("GET")
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
	s.assets.Styles = []string{}

	if len(entry.CSS) > 0 {
		s.assets.Styles = []string{"/static/" + entry.CSS[0]}
	}

	return nil
}

// Start runs the HTTP server in a separate goroutine.
func (s *Server) Start(ctx context.Context) error {
	const shutdownTimeout = 5 * time.Second

	frontendLog.Info("Starting Frontend server", "address", s.config.Addr)

	go func() {
		<-ctx.Done()
		frontendLog.Info("Shutting down Frontend server")

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()

		if err := s.server.Shutdown(shutdownCtx); err != nil {
			frontendLog.Error(err, "Frontend server shutdown error")
		}
	}()

	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		frontendLog.Error(err, "Frontend server error")

		return err
	}

	return nil
}
