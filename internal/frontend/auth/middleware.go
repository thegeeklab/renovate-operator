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

	headerAuthProvider = "X-Auth-Provider"
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
				WriteNotReadyResponse(w, r)

				return
			}

			if !manager.IsEnabled() {
				next.ServeHTTP(w, r)

				return
			}

			if IsAPIPath(r.URL.Path) {
				if handled := tryBearerTokenAuth(manager, w, r, next); handled {
					return
				}
			}

			if !handleCookieAuth(manager, w, r) {
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// tryBearerTokenAuth attempts to authenticate using Bearer token.
// Returns true if the request was handled (either successfully or with an error response).
func tryBearerTokenAuth(
	manager *Manager,
	w http.ResponseWriter,
	r *http.Request,
	next http.Handler,
) bool {
	token := extractBearerToken(r)
	if token == "" {
		return false
	}

	providerName := r.Header.Get(headerAuthProvider)
	if providerName == "" {
		writeUnauthorizedResponse(w, r, "missing "+headerAuthProvider+" header")

		return true
	}

	if !handleBearerTokenAuth(manager, w, r, next, token, providerName) {
		return true
	}

	return true
}

// handleBearerTokenAuth validates a Bearer token against the specified provider
// and injects the session into the request context.
// Returns true if the request should continue, false if a response was already written.
func handleBearerTokenAuth(
	manager *Manager,
	w http.ResponseWriter,
	r *http.Request,
	next http.Handler,
	token string,
	providerName string,
) bool {
	session, err := manager.validateBearerToken(r.Context(), token, providerName)
	if err != nil {
		authLog.Error(err, "Bearer token validation failed", "provider", providerName)
		writeUnauthorizedResponse(w, r, "invalid token")

		return false
	}

	ctx := SetAPISessionData(r.Context(), *session)
	next.ServeHTTP(w, r.WithContext(ctx))

	return true
}

// handleCookieAuth validates the cookie-based session and ensures the token is valid.
// Returns true if the request should continue, false if a response was already written.
func handleCookieAuth(
	manager *Manager,
	w http.ResponseWriter,
	r *http.Request,
) bool {
	sessionManager := manager.SessionManager()

	if !IsAuthenticated(r.Context(), sessionManager) {
		writeUnauthorizedResponse(w, r, "unauthorized")

		return false
	}

	providerName := GetProvider(r.Context(), sessionManager)

	_, ok := manager.Get(providerName)
	if !ok {
		writeUnauthorizedResponse(w, r, "invalid provider")

		return false
	}

	session, ok := GetSessionData(r.Context(), sessionManager)
	if !ok {
		writeUnauthorizedResponse(w, r, "unauthorized")

		return false
	}

	if !manager.ensureValidToken(w, r, sessionManager, providerName, session) {
		return false
	}

	return true
}

// extractBearerToken extracts the Bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	const bearerScheme = "bearer"
	if len(auth) < len(bearerScheme)+1 ||
		!strings.EqualFold(auth[:len(bearerScheme)], bearerScheme) ||
		auth[len(bearerScheme)] != ' ' {
		return ""
	}

	return auth[len(bearerScheme)+1:]
}

// validateBearerToken validates a Bearer token against the specified provider.
// Returns the session data if validation succeeds, or an error if the provider
// rejects the token or is not registered.
func (m *Manager) validateBearerToken(ctx context.Context, token, providerName string) (*SessionData, error) {
	cacheKey := providerName + ":" + HashAccessToken(token)
	if cached, found := m.patCache.GetIfPresent(cacheKey); found {
		return cached, nil
	}

	m.mu.RLock()
	provider, ok := m.providers[providerName]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrInvalidProvider
	}

	result, err, _ := m.validateGroup.Do(cacheKey, func() (any, error) {
		user, validateErr := provider.ValidateToken(ctx, token)
		if validateErr != nil {
			return nil, validateErr
		}

		if user == nil {
			return nil, ErrInvalidToken
		}

		session := &SessionData{
			Email:       user.Email,
			Name:        user.Name,
			Subject:     user.Subject,
			AvatarURL:   user.AvatarURL,
			AccessToken: user.AccessToken,
			Provider:    user.Provider,
		}

		m.patCache.Set(cacheKey, session)

		return session, nil
	})
	if err != nil {
		return nil, err
	}

	session, ok := result.(*SessionData)
	if !ok {
		return nil, ErrInvalidToken
	}

	return session, nil
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

func WriteNotReadyResponse(w http.ResponseWriter, r *http.Request) {
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
