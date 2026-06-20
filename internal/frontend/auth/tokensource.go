package auth

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

// SessionFunc is a callback function that retrieves the current session from the session store.
type SessionFunc func(ctx context.Context) *SessionData

// sessionTokenSource is an oauth2.TokenSource that reads the current access token
// from the session store on every call, so the token is always fresh and never
// stale across requests. It is bound to a request context to locate the session.
type sessionTokenSource struct {
	ctx         context.Context //nolint:containedctx // required to bind a TokenSource to the request's session
	sessionFunc SessionFunc
}

func (s *sessionTokenSource) Token() (*oauth2.Token, error) {
	session := s.sessionFunc(s.ctx)
	if session == nil || session.AccessToken == "" {
		return nil, ErrNoRefreshToken
	}

	return &oauth2.Token{
		AccessToken: session.AccessToken,
		TokenType:   "Bearer",
		Expiry:      session.TokenExpiry,
	}, nil
}

// TokenTransport is an http.RoundTripper that injects the current token from the
// session into outgoing requests. It delegates header injection to oauth2.Transport,
// reading the token fresh from the session store on each request to avoid stale tokens.
type TokenTransport struct {
	base        http.RoundTripper
	sessionFunc SessionFunc
}

// NewTokenTransport creates a new TokenTransport.
func NewTokenTransport(base http.RoundTripper, sessionFunc SessionFunc) *TokenTransport {
	if base == nil {
		base = http.DefaultTransport
	}

	return &TokenTransport{
		base:        base,
		sessionFunc: sessionFunc,
	}
}

// RoundTrip implements http.RoundTripper by delegating to oauth2.Transport with a
// per-request, session-backed token source.
func (t *TokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	oauthTransport := &oauth2.Transport{
		Base: t.base,
		Source: &sessionTokenSource{
			ctx:         req.Context(),
			sessionFunc: t.sessionFunc,
		},
	}

	return oauthTransport.RoundTrip(req)
}

// NewAuthClient creates an HTTP client that reads the current token from the session on each request.
// The client itself can be cached for connection pooling, but tokens are read fresh from the session.
func NewAuthClient(sessionFunc SessionFunc) *http.Client {
	transport := NewTokenTransport(http.DefaultTransport, sessionFunc)

	return &http.Client{
		Transport: transport,
	}
}
