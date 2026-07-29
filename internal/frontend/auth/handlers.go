package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	stateCookieName   = "renovate_oidc_state"
	stateCookieMaxAge = 300

	csrfFormName = "csrf_token"

	stateParts = 2
)

var authLog = logf.Log.WithName("auth")

// encodeState produces a state value (CSRF token + provider name) and a PKCE
// code verifier. The state is sent to the authorization server in the URL; the
// verifier is stored in the state cookie and sent only in the token exchange.
func encodeState(provider string) (string, string, error) {
	b := make([]byte, 32) //nolint:mnd
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}

	state := hex.EncodeToString(b) + ":" + base64.RawURLEncoding.EncodeToString([]byte(provider))
	verifier := oauth2.GenerateVerifier()

	return state, verifier, nil
}

// decodeStateCookie splits a cookie value into the CSRF state token and the
// PKCE code verifier. The cookie value is encoded as "<state>|<verifier>".
// Returns false if the value is malformed.
func decodeStateCookie(cookieValue string) (string, string, bool) {
	idx := strings.LastIndex(cookieValue, "|")
	if idx < 0 {
		return "", "", false
	}

	return cookieValue[:idx], cookieValue[idx+1:], true
}

// decodeState extracts the provider name from a state value previously produced by encodeState.
// It returns false if the state value is malformed.
func decodeState(state string) (string, bool) {
	parts := strings.SplitN(state, ":", stateParts)
	if len(parts) != stateParts {
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
		if manager.IsIntended() && !manager.IsEnabled() {
			WriteNotReadyResponse(w, r)

			return
		}

		if !manager.IsEnabled() {
			http.Redirect(w, r, "/", http.StatusFound)

			return
		}

		if IsAuthenticated(r.Context(), manager.SessionManager()) {
			http.Redirect(w, r, "/", http.StatusFound)

			return
		}

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

		state, verifier, err := encodeState(providerName)
		if err != nil {
			http.Error(w, "failed to generate state", http.StatusInternalServerError)

			return
		}

		http.SetCookie(w, &http.Cookie{ //nolint:gosec
			Name:     stateCookieName,
			Value:    state + "|" + verifier,
			Path:     "/auth/callback",
			MaxAge:   stateCookieMaxAge,
			HttpOnly: true,
			Secure:   isSecureRequest(r, secureCookies),
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, provider.LoginURL(state, verifier), http.StatusFound)
	}
}

func HandleCallback(manager *Manager, secureCookies bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if manager.IsIntended() && !manager.IsEnabled() {
			WriteNotReadyResponse(w, r)

			return
		}

		stateCookie, err := r.Cookie(stateCookieName)
		if err != nil {
			http.Error(w, "missing state cookie", http.StatusBadRequest)

			return
		}

		state, verifier, ok := decodeStateCookie(stateCookie.Value)
		if !ok {
			http.Error(w, "invalid state cookie", http.StatusBadRequest)

			return
		}

		urlState := r.URL.Query().Get("state")
		if urlState != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)

			return
		}

		providerName, ok := decodeState(urlState)
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

		user, err := provider.HandleCallback(r.Context(), code, verifier)
		if err != nil {
			authLog.Error(err, "OIDC callback failed")
			http.Error(w, "authentication failed", http.StatusInternalServerError)

			return
		}

		session := SessionData{
			Email:        user.Email,
			Name:         user.Name,
			Subject:      user.Subject,
			AvatarURL:    user.AvatarURL,
			AccessToken:  user.AccessToken,
			RefreshToken: user.RefreshToken,
			TokenExpiry:  user.TokenExpiry,
			Provider:     providerName,
		}

		sessionManager := manager.SessionManager()

		if err := sessionManager.RenewToken(r.Context()); err != nil {
			authLog.Error(err, "Failed to renew session token")
		}

		SetSessionData(r.Context(), sessionManager, session)

		if _, err := GenerateCSRFToken(r.Context(), sessionManager); err != nil {
			authLog.Error(err, "Failed to generate CSRF token")
		}

		secure := isSecureRequest(r, secureCookies)

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

func HandleLogout(manager *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionManager := manager.SessionManager()

		if IsAuthenticated(r.Context(), sessionManager) {
			if !ValidateCSRFToken(r.Context(), sessionManager, r.FormValue(csrfFormName)) {
				http.Error(w, "invalid CSRF token", http.StatusForbidden)

				return
			}
		}

		if err := DestroySession(r.Context(), sessionManager); err != nil {
			authLog.Error(err, "Failed to destroy session")
		}

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func HandleAuthStatus(manager *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if manager.IsIntended() && !manager.IsEnabled() {
			_, _ = w.Write([]byte(`{"enabled":true,"ready":false}`))

			return
		}

		if !manager.IsEnabled() {
			_, _ = w.Write([]byte(`{"enabled":false}`))

			return
		}

		sessionManager := manager.SessionManager()

		if !IsAuthenticated(r.Context(), sessionManager) {
			_, _ = w.Write([]byte(`{"enabled":true,"authenticated":false}`))

			return
		}

		session, _ := GetSessionData(r.Context(), sessionManager)

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
			"avatarURL":     session.AvatarURL,
			"provider":      provider.Name(),
			"providerType":  provider.Type(),
		}

		if err := json.NewEncoder(w).Encode(status); err != nil {
			authLog.Error(err, "Failed to encode auth status")
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
