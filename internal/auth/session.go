package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
)

var errSessionSecretRequired = errors.New("session secret must not be empty")

const (
	sessionCookieName   = "renovate_session"
	sessionDuration     = 24 * time.Hour
	sessionKeyEmail     = "email"
	sessionKeyName      = "name"
	sessionKeySubject   = "sub"
	sessionKeyToken     = "accessToken"
	sessionKeyProvider  = "provider"
	sessionKeyCSRFToken = "_csrf"
)

type SessionData struct {
	Email       string
	Name        string
	Subject     string
	AccessToken string
	Provider    string
}

func NewSessionManager(secret string, secureCookies bool) (*scs.SessionManager, error) {
	if secret == "" {
		return nil, errSessionSecretRequired
	}

	session := scs.New()
	session.Lifetime = sessionDuration
	session.Cookie.Name = sessionCookieName
	session.Cookie.HttpOnly = true
	session.Cookie.Secure = secureCookies
	session.Cookie.SameSite = http.SameSiteLaxMode
	session.Cookie.Path = "/"

	return session, nil
}

func SetSessionData(ctx context.Context, session *scs.SessionManager, data SessionData) {
	session.Put(ctx, sessionKeyEmail, data.Email)
	session.Put(ctx, sessionKeyName, data.Name)
	session.Put(ctx, sessionKeySubject, data.Subject)
	session.Put(ctx, sessionKeyToken, data.AccessToken)
	session.Put(ctx, sessionKeyProvider, data.Provider)
}

func GetSessionData(ctx context.Context, session *scs.SessionManager) (SessionData, bool) {
	if !session.Exists(ctx, sessionKeyProvider) {
		return SessionData{}, false
	}

	return SessionData{
		Email:       session.GetString(ctx, sessionKeyEmail),
		Name:        session.GetString(ctx, sessionKeyName),
		Subject:     session.GetString(ctx, sessionKeySubject),
		AccessToken: session.GetString(ctx, sessionKeyToken),
		Provider:    session.GetString(ctx, sessionKeyProvider),
	}, true
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
