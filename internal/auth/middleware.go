package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const sessionContextKey contextKey = "session"

func Middleware(manager *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !manager.IsEnabled() {
				next.ServeHTTP(w, r)

				return
			}

			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)

				return
			}

			session, err := getSessionFromRequest(r)
			if err != nil {
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

			_, ok := manager.Get(session.Provider)
			if !ok {
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

			ctx := contextWithSession(r.Context(), session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isPublicPath(path string) bool {
	if strings.HasPrefix(path, "/auth/") {
		return true
	}

	switch path {
	case "/health", "/healthz", "/readyz", "/login":
		return true
	default:
		return false
	}
}

func isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/") || path == "/events"
}

func contextWithSession(ctx context.Context, session SessionData) context.Context {
	return context.WithValue(ctx, sessionContextKey, session)
}

func SessionFromContext(ctx context.Context) (SessionData, bool) {
	session, ok := ctx.Value(sessionContextKey).(SessionData)

	return session, ok
}

func IsAuthenticated(ctx context.Context) bool {
	_, ok := SessionFromContext(ctx)

	return ok
}
