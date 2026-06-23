package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"golang.org/x/oauth2"
)

// HashAccessToken creates a stable hash of the access token for use in cache keys.
// It is shared so that all token-derived cache/singleflight keys stay in lockstep.
func HashAccessToken(token string) string {
	hash := sha256.Sum256([]byte(token))

	return hex.EncodeToString(hash[:])
}

const (
	errorTitleNotReady = "Service Unavailable"
	errorMsgNotReady   = "Authentication service is not ready yet. Please try again later."
)

func Middleware(manager *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		handler := authCheckMiddleware(manager)(next)

		return manager.SessionManager().LoadAndSave(handler)
	}
}

// authCheckMiddleware validates authentication state and handles token refresh.
// It checks if auth is intended but not ready, validates the session exists,
// and ensures tokens are valid (refreshing if expired).
func authCheckMiddleware(manager *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if IsPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)

				return
			}

			if manager.IsIntended() && !manager.IsEnabled() {
				writeNotReadyResponse(w, r)

				return
			}

			if !manager.IsEnabled() {
				next.ServeHTTP(w, r)

				return
			}

			sessionManager := manager.SessionManager()

			if !IsAuthenticated(r.Context(), sessionManager) {
				writeUnauthorizedResponse(w, r, "unauthorized")

				return
			}

			providerName := GetProvider(r.Context(), sessionManager)

			_, ok := manager.Get(providerName)
			if !ok {
				writeUnauthorizedResponse(w, r, "invalid provider")

				return
			}

			session, ok := GetSessionData(r.Context(), sessionManager)
			if !ok {
				writeUnauthorizedResponse(w, r, "unauthorized")

				return
			}

			if !manager.ensureValidToken(w, r, sessionManager, providerName, session) {
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ensureValidToken checks if the session token is valid and refreshes it if expired.
// Returns true if the token is valid or was successfully refreshed, false otherwise.
func (m *Manager) ensureValidToken(
	w http.ResponseWriter,
	r *http.Request,
	sessionManager *scs.SessionManager,
	providerName string,
	session SessionData,
) bool {
	if !session.TokenExpired() {
		return true
	}

	if session.RefreshToken == "" {
		authLog.Info("Token expired with no refresh token, forcing re-authentication")

		if err := DestroySession(r.Context(), sessionManager); err != nil {
			authLog.Error(err, "Failed to destroy session after token expiry")
		}

		writeUnauthorizedResponse(w, r, "token expired")

		return false
	}

	// Use singleflight to prevent thundering herd on token refresh
	// Use hashed access token as fallback when Subject is empty to avoid cross-user collision
	refreshKey := "refresh:" + session.Provider + ":" + session.Subject
	if session.Subject == "" {
		refreshKey = "refresh:" + session.Provider + ":token:" + HashAccessToken(session.AccessToken)
	}

	// Use context.WithoutCancel to prevent first caller's cancellation from affecting others
	ctx := context.WithoutCancel(r.Context())

	result, err, _ := m.refreshGroup.Do(refreshKey, func() (any, error) {
		return m.RefreshSessionToken(ctx, &session)
	})
	if err != nil {
		// Check if this is a transient error (network, 5xx) vs permanent (invalid refresh token)
		if isTransientError(err) {
			authLog.Error(err, "Token refresh failed due to transient error, preserving session")
			writeUnauthorizedResponse(w, r, "token refresh temporarily unavailable")

			return false
		}

		authLog.Error(err, "Token refresh failed, forcing re-authentication")

		if err := DestroySession(r.Context(), sessionManager); err != nil {
			authLog.Error(err, "Failed to destroy session after refresh failure")
		}

		writeUnauthorizedResponse(w, r, "token refresh failed")

		return false
	}

	updatedSession, ok := result.(*SessionData)
	if !ok {
		authLog.Error(nil, "Token refresh returned unexpected type")

		if err := DestroySession(r.Context(), sessionManager); err != nil {
			authLog.Error(err, "Failed to destroy session after refresh failure")
		}

		writeUnauthorizedResponse(w, r, "token refresh failed")

		return false
	}

	SetSessionData(r.Context(), sessionManager, *updatedSession)

	authLog.Info("Token refreshed successfully", "provider", providerName, "subject", updatedSession.Subject)

	return true
}

// isTransientError reports whether an error is transient (network blip, rate
// limit, or 5xx) as opposed to permanent (e.g. an invalid/revoked refresh
// token). It relies on typed errors rather than matching error message text,
// which is brittle and can both keep a dead session alive and destroy a valid
// one. The decision determines whether the user's session is preserved or
// destroyed, so a permanent default (false) is the safe, fail-closed choice.
func isTransientError(err error) bool {
	// Context cancellation/deadline are transient.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	// OAuth2 token endpoint errors carry the HTTP status. 429 and 5xx are
	// transient; 4xx (e.g. invalid_grant on a revoked refresh token) are permanent.
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) && retrieveErr.Response != nil {
		code := retrieveErr.Response.StatusCode

		return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
	}

	// Network-level errors that report themselves as timeouts are transient.
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	// Unrecognized errors are treated as permanent (fail closed): force
	// re-authentication rather than indefinitely preserving a possibly dead session.
	return false
}

func writeUnauthorizedResponse(w http.ResponseWriter, r *http.Request, message string) {
	if IsAPIPath(r.URL.Path) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)

		if err := json.NewEncoder(w).Encode(map[string]string{
			"error": message,
		}); err != nil {
			authLog.Error(err, "Failed to encode unauthorized response")
		}

		return
	}

	http.Redirect(w, r, "/login", http.StatusFound)
}

// IsPublicPath returns true if the path does not require authentication.
func IsPublicPath(path string) bool {
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

// IsAPIPath returns true if the path is an API endpoint.
func IsAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/")
}

func writeNotReadyResponse(w http.ResponseWriter, r *http.Request) {
	if IsAPIPath(r.URL.Path) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)

		if err := json.NewEncoder(w).Encode(map[string]string{
			"error": "auth not ready",
		}); err != nil {
			authLog.Error(err, "Failed to encode auth not ready response")
		}

		return
	}

	w.Header().Set("X-Error-Title", errorTitleNotReady)
	w.Header().Set("X-Error-Message", errorMsgNotReady)
	w.WriteHeader(http.StatusServiceUnavailable)
}
