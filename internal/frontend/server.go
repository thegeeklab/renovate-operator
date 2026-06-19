package frontend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/thegeeklab/renovate-operator/internal/auth"
	"github.com/thegeeklab/renovate-operator/internal/frontend/view"
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

	if authManager != nil {
		s.registerAuthRoutes()
	}

	if !s.config.DevMode {
		staticDir := "internal/frontend/static/dist"
		fs := http.FileServer(http.Dir(staticDir))
		s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	}

	var handler http.Handler = s.router

	if authManager != nil {
		handler = auth.Middleware(authManager)(s.router)
		handler = s.errorPageMiddleware(handler)
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

// ErrorPageInfo contains the content for a styled error page.
type ErrorPageInfo struct {
	Title   string
	Message string
}

// defaultErrorPages provides fallback content for common HTTP error codes.
// Components can override these by setting X-Error-Title and X-Error-Message headers.
var defaultErrorPages = map[int]ErrorPageInfo{
	http.StatusServiceUnavailable: {
		Title:   "Service Unavailable",
		Message: "The service is temporarily unavailable. Please try again later.",
	},
	http.StatusUnauthorized: {
		Title:   "Unauthorized",
		Message: "You need to log in to access this resource.",
	},
	http.StatusForbidden: {
		Title:   "Forbidden",
		Message: "You don't have permission to access this resource.",
	},
	http.StatusNotFound: {
		Title:   "Not Found",
		Message: "The requested resource could not be found.",
	},
}

// errorPageMiddleware wraps the handler to intercept error responses and render styled error pages.
// It checks for X-Error-Title and X-Error-Message headers first, falling back to generic defaults.
func (s *Server) errorPageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           &bytes.Buffer{},
		}
		next.ServeHTTP(recorder, r)

		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.WriteHeader(recorder.statusCode)
			_, _ = w.Write(recorder.body.Bytes())

			return
		}

		if page, ok := defaultErrorPages[recorder.statusCode]; ok {
			title := recorder.Header().Get("X-Error-Title")
			message := recorder.Header().Get("X-Error-Message")

			if title == "" {
				title = page.Title
			}

			if message == "" {
				message = page.Message
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(recorder.statusCode)

			err := view.ErrorPage(recorder.statusCode, title, message, s.assets.Styles).Render(r.Context(), w)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}

			return
		}

		w.WriteHeader(recorder.statusCode)
		_, _ = w.Write(recorder.body.Bytes())
	})
}

// statusRecorder wraps http.ResponseWriter to capture the status code and buffer the response body.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	return r.body.Write(b)
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
