package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
)

const (
	sessionCookieName      = "renovate_session"
	sessionDuration        = 24 * time.Hour
	sessionKeyEmail        = "email"
	sessionKeyName         = "name"
	sessionKeySubject      = "sub"
	sessionKeyToken        = "accessToken"
	sessionKeyRefreshToken = "refreshToken"
	sessionKeyTokenExpiry  = "tokenExpiry"
	sessionKeyIssuedAt     = "issuedAt"
	sessionKeyProvider     = "provider"
	sessionKeyAvatarURL    = "avatarURL"
	sessionKeyCSRFToken    = "_csrf"

	TokenExpiryBuffer = 30 * time.Second

	// defaultTokenLifetime is the assumed access-token lifetime used only when the
	// provider does not return an expiry (no expires_in). It bounds how often a
	// token without a known expiry is refreshed, preventing a refresh on every request.
	defaultTokenLifetime = 1 * time.Hour
)

type SessionData struct {
	Email        string
	Name         string
	Subject      string
	AvatarURL    string
	AccessToken  string
	RefreshToken string
	TokenExpiry  time.Time
	IssuedAt     time.Time
	Provider     string
}

// effectiveExpiry returns the token expiry, synthesizing one from IssuedAt and a
// default lifetime when the provider did not return an explicit expiry. This
// prevents a refresh on every request for providers that omit expires_in.
func (s SessionData) effectiveExpiry() time.Time {
	if !s.TokenExpiry.IsZero() {
		return s.TokenExpiry
	}

	if !s.IssuedAt.IsZero() {
		return s.IssuedAt.Add(defaultTokenLifetime)
	}

	return time.Time{}
}

func (s SessionData) TokenExpired() bool {
	expiry := s.effectiveExpiry()

	// Expiry is genuinely unknown (legacy session with no IssuedAt). Force a single
	// refresh when a refresh token is available so the session adopts a known expiry.
	if expiry.IsZero() {
		return s.RefreshToken != ""
	}

	return time.Now().Add(TokenExpiryBuffer).After(expiry)
}

func NewSessionManager(secureCookies bool) *scs.SessionManager {
	session := scs.New()
	session.Lifetime = sessionDuration
	session.Cookie.Name = sessionCookieName
	session.Cookie.HttpOnly = true
	session.Cookie.Secure = secureCookies
	session.Cookie.SameSite = http.SameSiteLaxMode
	session.Cookie.Path = "/"

	// TODO: Allow optional redisstore
	session.Store = memstore.New()

	authLog.Info(
		"Session store is in-memory: sessions do not survive restarts and the " +
			"frontend must run with a single replica. Configure a shared store to scale out.",
	)

	return session
}

func SetSessionData(ctx context.Context, session *scs.SessionManager, data SessionData) {
	session.Put(ctx, sessionKeyEmail, data.Email)
	session.Put(ctx, sessionKeyName, data.Name)
	session.Put(ctx, sessionKeySubject, data.Subject)
	session.Put(ctx, sessionKeyToken, data.AccessToken)
	session.Put(ctx, sessionKeyRefreshToken, data.RefreshToken)

	if !data.TokenExpiry.IsZero() {
		session.Put(ctx, sessionKeyTokenExpiry, data.TokenExpiry.Format(time.RFC3339Nano))
	} else {
		session.Remove(ctx, sessionKeyTokenExpiry)
	}

	// Record when this token was stored. Used to synthesize an expiry for providers
	// that do not return expires_in, bounding refresh frequency.
	issuedAt := data.IssuedAt
	if issuedAt.IsZero() {
		issuedAt = time.Now()
	}

	session.Put(ctx, sessionKeyIssuedAt, issuedAt.Format(time.RFC3339Nano))

	session.Put(ctx, sessionKeyProvider, data.Provider)
	session.Put(ctx, sessionKeyAvatarURL, data.AvatarURL)
}

func GetSessionData(ctx context.Context, session *scs.SessionManager) (SessionData, bool) {
	if !session.Exists(ctx, sessionKeyProvider) {
		return SessionData{}, false
	}

	data := SessionData{
		Email:        session.GetString(ctx, sessionKeyEmail),
		Name:         session.GetString(ctx, sessionKeyName),
		Subject:      session.GetString(ctx, sessionKeySubject),
		AvatarURL:    session.GetString(ctx, sessionKeyAvatarURL),
		AccessToken:  session.GetString(ctx, sessionKeyToken),
		RefreshToken: session.GetString(ctx, sessionKeyRefreshToken),
		Provider:     session.GetString(ctx, sessionKeyProvider),
	}

	if issuedStr := session.GetString(ctx, sessionKeyIssuedAt); issuedStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, issuedStr); err == nil {
			data.IssuedAt = t
		}
	}

	if expiryStr := session.GetString(ctx, sessionKeyTokenExpiry); expiryStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, expiryStr); err == nil {
			data.TokenExpiry = t
		}
	}

	return data, true
}

func GetProvider(ctx context.Context, session *scs.SessionManager) string {
	return session.GetString(ctx, sessionKeyProvider)
}

func IsAuthenticated(ctx context.Context, session *scs.SessionManager) bool {
	return session.Exists(ctx, sessionKeyProvider)
}

func DestroySession(ctx context.Context, session *scs.SessionManager) error {
	return session.Destroy(ctx)
}

func GenerateCSRFToken(ctx context.Context, session *scs.SessionManager) (string, error) {
	b := make([]byte, 32) //nolint:mnd
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	token := hex.EncodeToString(b)
	session.Put(ctx, sessionKeyCSRFToken, token)

	return token, nil
}

func GetCSRFToken(ctx context.Context, session *scs.SessionManager) string {
	return session.GetString(ctx, sessionKeyCSRFToken)
}

func ValidateCSRFToken(ctx context.Context, session *scs.SessionManager, token string) bool {
	expected := session.GetString(ctx, sessionKeyCSRFToken)

	return expected != "" && token != "" && expected == token
}
