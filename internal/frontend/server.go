package frontend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/thegeeklab/renovate-operator/internal/frontend/auth"
	"github.com/thegeeklab/renovate-operator/internal/frontend/i18n"
	"github.com/thegeeklab/renovate-operator/internal/frontend/view"
	"github.com/thegeeklab/renovate-operator/internal/frontend/viewmodel"
	"github.com/thegeeklab/renovate-operator/internal/logreader"
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
		Addr:         ":8082",
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
	router      chi.Router
	server      *http.Server
	apiHandler  *APIHandler
	webHandler  *WebHandler
	authManager *auth.Manager
	i18nBundle  *goi18n.Bundle
}

// NewServer creates a new HTTP server instance.
func NewServer(
	config ServerConfig,
	client client.Client,
	clientset kubernetes.Interface,
	broker *SSEBroker,
	authManager *auth.Manager,
	logReader logreader.Reader,
) *Server {
	s := &Server{
		config:      config,
		router:      chi.NewRouter(),
		authManager: authManager,
		i18nBundle:  i18n.NewBundle(),
	}

	if err := s.loadFrontendAssets(); err != nil {
		frontendLog.Error(err, "Failed to load frontend assets (UI might be broken)")
	} else {
		frontendLog.Info("Frontend assets loaded", "devMode", s.config.DevMode)
	}

	s.apiHandler = NewAPIHandler(client, clientset, authManager, logReader)
	s.webHandler = NewWebHandler(client, clientset, broker, s.assets, authManager, logReader)

	s.router.Use(i18n.Middleware(s.i18nBundle, i18n.MiddlewareConfig{SecureCookies: s.config.SecureCookies}))
	s.router.Use(errorPageMiddleware(s.assets.Styles, s.assets.Scripts, authManager))

	if authManager != nil {
		s.router.Use(auth.Middleware(authManager))
	}

	s.apiHandler.RegisterRoutes(s.router)
	s.webHandler.RegisterRoutes(s.router)

	if authManager != nil {
		s.registerAuthRoutes()
	}

	staticDir := "internal/frontend/static/dist"
	if s.config.DevMode {
		staticDir = "internal/frontend/static/public"
	}

	fs := http.FileServer(http.Dir(staticDir))
	s.router.Handle("/static/*", http.StripPrefix("/static/", fs))

	s.server = &http.Server{
		Addr:         config.Addr,
		Handler:      s.router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return s
}

func (s *Server) registerAuthRoutes() {
	s.router.Get("/auth/login", auth.HandleLogin(s.authManager, s.config.SecureCookies))
	s.router.Get("/auth/callback", auth.HandleCallback(s.authManager, s.config.SecureCookies))
	s.router.Post("/auth/logout", auth.HandleLogout(s.authManager))
	s.router.Get("/api/v1/auth/status", auth.HandleAuthStatus(s.authManager))
}

// errorPageMiddleware intercepts error responses and renders styled error pages for browser requests
// while passing through responses unchanged for API requests. It uses a response writer wrapper to
// buffer the body and capture the status code, then decides whether to render a styled page based
// on the request path and status.
func errorPageMiddleware(
	styles []string,
	scripts []string,
	authManager *auth.Manager,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth.IsAPIPath(r.URL.Path) {
				next.ServeHTTP(w, r)

				return
			}

			rec := &errorStatusRecorder{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
			}

			next.ServeHTTP(rec, r)

			if rec.isStreaming() {
				return
			}

			if rec.statusCode < http.StatusBadRequest {
				rec.commit()

				if _, err := w.Write(rec.body.Bytes()); err != nil {
					frontendLog.Error(err, "Failed to write response body")
				}

				return
			}

			title, message := resolveErrorInfo(r.Context(), rec, rec.statusCode)

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("X-Page-Title", url.PathEscape(title))
			rec.commit()

			authInfo := buildErrorAuthInfo(r, authManager)

			if err := view.ErrorPage(
				r.Context(),
				rec.statusCode, title, message, title, styles, scripts, authInfo,
			).Render(r.Context(), w); err != nil {
				frontendLog.Error(err, "Failed to render error page")
			}
		})
	}
}

func buildErrorAuthInfo(r *http.Request, authManager *auth.Manager) viewmodel.AuthInfo {
	info := viewmodel.AuthInfo{}

	if authManager == nil || !authManager.IsEnabled() {
		return info
	}

	info.Enabled = true

	sessionManager := authManager.SessionManager()

	var token string
	if cookie, err := r.Cookie(sessionManager.Cookie.Name); err == nil {
		token = cookie.Value
	}

	ctx, err := sessionManager.Load(r.Context(), token)
	if err != nil {
		return info
	}

	session, ok := auth.GetSessionData(ctx, sessionManager)
	if !ok {
		return info
	}

	info.Authenticated = true
	info.Name = session.Name
	info.AvatarURL = session.AvatarURL
	info.Provider = session.Provider

	csrfToken := auth.GetCSRFToken(ctx, sessionManager)
	if csrfToken != "" {
		info.CSRFToken = csrfToken
	}

	return info
}

// resolveErrorInfo determines the error title and message to display.
// Headers X-Error-Title and X-Error-Message from upstream handlers take precedence.
// Otherwise default translated messages are used based on the HTTP status code.
func resolveErrorInfo(ctx context.Context, rec *errorStatusRecorder, code int) (string, string) {
	title := rec.Header().Get("X-Error-Title")
	message := rec.Header().Get("X-Error-Message")
	tr := i18n.FromContext(ctx)

	if title != "" || message != "" {
		if title == "" {
			title = http.StatusText(code)
		}

		return title, message
	}

	switch code {
	case http.StatusServiceUnavailable:
		return tr.T("error.service_unavailable"), tr.T("error.service_unavailable_message")
	case http.StatusUnauthorized:
		return tr.T("error.unauthorized"), tr.T("error.unauthorized_message")
	case http.StatusForbidden:
		return tr.T("error.forbidden"), tr.T("error.forbidden_message")
	case http.StatusNotFound:
		return tr.T("error.not_found"), tr.T("error.not_found_message")
	}

	return http.StatusText(code), tr.T("error.unexpected")
}

// errorStatusRecorder wraps http.ResponseWriter to capture the status code and buffer the response body.
// Streaming responses (text/event-stream) bypass buffering and write directly to the underlying writer.
// It implements http.Hijacker to support WebSocket upgrades and other protocols requiring connection hijacking.
type errorStatusRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	committed  bool
}

func (r *errorStatusRecorder) isStreaming() bool {
	return r.Header().Get("Content-Type") == "text/event-stream"
}

func (r *errorStatusRecorder) commit() {
	if r.committed {
		return
	}

	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}

	r.ResponseWriter.WriteHeader(r.statusCode)
	r.committed = true
}

func (r *errorStatusRecorder) WriteHeader(code int) {
	if r.statusCode == 0 {
		r.statusCode = code
	}

	if r.isStreaming() {
		r.commit()
	}
}

func (r *errorStatusRecorder) Write(b []byte) (int, error) {
	if r.isStreaming() {
		r.commit()

		return r.ResponseWriter.Write(b)
	}

	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}

	return r.body.Write(b)
}

func (r *errorStatusRecorder) Flush() {
	r.commit()

	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *errorStatusRecorder) SetWriteDeadline(t time.Time) error {
	type dl interface {
		SetWriteDeadline(time.Time) error
	}

	if d, ok := r.ResponseWriter.(dl); ok {
		return d.SetWriteDeadline(t)
	}

	return http.ErrNotSupported
}

// Hijack implements http.Hijacker to support WebSocket upgrades and other protocols
// requiring connection hijacking. It delegates to the underlying ResponseWriter.
func (r *errorStatusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := r.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}

	return nil, nil, http.ErrNotSupported
}

// loadFrontendAssets populates the server's FrontendAssets struct based on the configuration.
func (s *Server) loadFrontendAssets() error {
	if s.config.DevMode {
		s.assets.Scripts = []string{
			"http://localhost:5173/@vite/client",
			"http://localhost:5173/internal/frontend/static/main.ts",
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

	entry, ok := manifest["internal/frontend/static/main.ts"]
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
