package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	stateCookieName   = "renovate_oidc_state"
	stateCookieMaxAge = 300
)

var authLog = log.Log.WithName("auth")

// encodeState produces a state value of the form "<random-hex>:<base64url-providername>".
// The random component serves as the CSRF token (compared against the cookie),
// while the provider segment binds the provider identity to the state so the
// callback handler does not need to trust an attacker-controlled URL parameter.
func encodeState(provider string) (string, error) {
	b := make([]byte, 32) //nolint:mnd
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b) + ":" + base64.RawURLEncoding.EncodeToString([]byte(provider)), nil
}

// decodeState extracts the provider name from a state value previously produced by encodeState.
// It returns false if the state value is malformed.
func decodeState(state string) (provider string, ok bool) {
	parts := strings.SplitN(state, ":", 2)
	if len(parts) != 2 {
		return "", false
	}

	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}

	return string(b), true
}

// isSecureRequest determines whether a request was made over a secure transport,
// honoring the explicit secureCookies override flag, direct TLS, and reverse proxy
// X-Forwarded-Proto headers.
func isSecureRequest(r *http.Request, secureCookies bool) bool {
	return secureCookies || r.TLS != nil ||
		strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func HandleLogin(manager *Manager, secureCookies bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerName := r.URL.Query().Get("provider")
		if providerName == "" {
			http.Error(w, "provider parameter required", http.StatusBadRequest)

			return
		}

		provider, ok := manager.Get(providerName)
		if !ok {
			http.Error(w, "unknown provider", http.StatusNotFound)

			return
		}

		state, err := encodeState(providerName)
		if err != nil {
			http.Error(w, "failed to generate state", http.StatusInternalServerError)

			return
		}

		http.SetCookie(w, &http.Cookie{ //nolint:gosec
			Name:     stateCookieName,
			Value:    state,
			Path:     "/auth/callback",
			MaxAge:   stateCookieMaxAge,
			HttpOnly: true,
			Secure:   isSecureRequest(r, secureCookies),
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, provider.LoginURL(state), http.StatusFound)
	}
}

func HandleCallback(manager *Manager, secureCookies bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stateCookie, err := r.Cookie(stateCookieName)
		if err != nil {
			http.Error(w, "missing state cookie", http.StatusBadRequest)

			return
		}

		state := r.URL.Query().Get("state")
		if state != stateCookie.Value {
			http.Error(w, "state mismatch", http.StatusBadRequest)

			return
		}

		providerName, ok := decodeState(state)
		if !ok {
			http.Error(w, "invalid state", http.StatusBadRequest)

			return
		}

		provider, ok := manager.Get(providerName)
		if !ok {
			http.Error(w, "unknown provider", http.StatusNotFound)

			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing authorization code", http.StatusBadRequest)

			return
		}

		user, err := provider.HandleCallback(r.Context(), code)
		if err != nil {
			authLog.Error(err, "OIDC callback failed")
			http.Error(w, "authentication failed", http.StatusInternalServerError)

			return
		}

		session := SessionData{
			Email:       user.Email,
			Name:        user.Name,
			Subject:     user.Subject,
			AccessToken: user.AccessToken,
			Provider:    user.Provider,
			Expiry:      sessionDataExpiry(),
		}

		encrypted, err := encryptSession(session)
		if err != nil {
			http.Error(w, "failed to create session", http.StatusInternalServerError)

			return
		}

		secure := isSecureRequest(r, secureCookies)

		http.SetCookie(w, &http.Cookie{ //nolint:gosec
			Name:     sessionCookieName,
			Value:    encrypted,
			Path:     "/",
			MaxAge:   int(sessionDuration.Seconds()),
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})

		http.SetCookie(w, &http.Cookie{ //nolint:gosec
			Name:     stateCookieName,
			Value:    "",
			Path:     "/auth/callback",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func HandleLogout(secureCookies bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secure := isSecureRequest(r, secureCookies)

		http.SetCookie(w, &http.Cookie{ //nolint:gosec
			Name:     sessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func HandleAuthStatus(manager *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if !manager.IsEnabled() {
			_, _ = w.Write([]byte(`{"enabled":false}`))

			return
		}

		session, err := getSessionFromRequest(r)
		if err != nil {
			_, _ = w.Write([]byte(`{"enabled":true,"authenticated":false}`))

			return
		}

		provider, ok := manager.Get(session.Provider)
		if !ok {
			_, _ = w.Write([]byte(`{"enabled":true,"authenticated":false}`))

			return
		}

		status := map[string]any{
			"enabled":       true,
			"authenticated": true,
			"email":         session.Email,
			"name":          session.Name,
			"provider":      provider.Name(),
			"providerType":  provider.Type(),
		}

		if err := json.NewEncoder(w).Encode(status); err != nil {
			authLog.Error(err, "Failed to encode auth status")
		}
	}
}

func getSessionFromRequest(r *http.Request) (SessionData, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return SessionData{}, err
	}

	return decryptSession(cookie.Value)
}

func sessionDataExpiry() time.Time {
	return time.Now().Add(sessionDuration)
}
