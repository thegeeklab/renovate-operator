package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

const (
	errorTitleNotReady = "Service Unavailable"
	errorMsgNotReady   = "Authentication service is not ready yet. Please try again later."
)

func Middleware(manager *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		sessionManager := manager.Session

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if manager.IsIntended() && !manager.IsEnabled() {
				if isAPIPath(r.URL.Path) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusServiceUnavailable)

					_ = json.NewEncoder(w).Encode(map[string]string{
						"error": "auth not ready",
					})

					return
				}

				w.Header().Set("X-Error-Title", errorTitleNotReady)
				w.Header().Set("X-Error-Message", errorMsgNotReady)
				w.WriteHeader(http.StatusServiceUnavailable)

				return
			}

			if !manager.IsEnabled() {
				next.ServeHTTP(w, r)

				return
			}

			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)

				return
			}

			if !IsAuthenticated(r.Context(), sessionManager) {
				if isAPIPath(r.URL.Path) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)

					_ = json.NewEncoder(w).Encode(map[string]string{
						"error": "unauthorized",
					})

					return
				}

				http.Redirect(w, r, "/login", http.StatusFound)

				return
			}

			providerName := GetProvider(r.Context(), sessionManager)
			if _, ok := manager.Get(providerName); !ok {
				if isAPIPath(r.URL.Path) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)

					_ = json.NewEncoder(w).Encode(map[string]string{
						"error": "invalid provider",
					})

					return
				}

				http.Redirect(w, r, "/login", http.StatusFound)

				return
			}

			next.ServeHTTP(w, r)
		})

		return sessionManager.LoadAndSave(handler)
	}
}

func isPublicPath(path string) bool {
	if strings.HasPrefix(path, "/static/") {
		return true
	}

	switch path {
	case "/auth/login", "/auth/callback", "/auth/logout",
		"/health", "/healthz", "/readyz", "/login",
		"/api/v1/auth/status":
		return true
	default:
		return false
	}
}

func isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/")
}
