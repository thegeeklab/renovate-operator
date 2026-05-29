package gitea

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/thegeeklab/renovate-operator/internal/auth"
	"golang.org/x/oauth2"
)

var errNoIDToken = errors.New("no id_token in token response")

type GiteaProvider struct {
	name         string
	issuerURL    string
	clientID     string
	clientSecret string
	redirectURL  string
	forgeURL     string
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	httpClient   *http.Client
}

func NewGiteaProvider(cfg auth.ProviderConfig) (*GiteaProvider, error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.Insecure}, //nolint:gosec
		},
	}

	ctx := oidc.ClientContext(context.Background(), httpClient)

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OIDC provider: %w", err)
	}

	oidcConfig := &oidc.Config{ClientID: cfg.ClientID}

	return &GiteaProvider{
		name:         cfg.Name,
		issuerURL:    cfg.IssuerURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURL:  cfg.RedirectURL,
		forgeURL:     cfg.ForgeURL,
		provider:     provider,
		verifier:     provider.Verifier(oidcConfig),
		oauth2Config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
		httpClient: httpClient,
	}, nil
}

func (p *GiteaProvider) Type() string {
	return auth.ProviderTypeGitea
}

func (p *GiteaProvider) Name() string {
	return p.name
}

func (p *GiteaProvider) ForgeURL() string {
	return p.forgeURL
}

func (p *GiteaProvider) LoginURL(state string) string {
	return p.oauth2Config.AuthCodeURL(state)
}

func (p *GiteaProvider) HandleCallback(ctx context.Context, code string) (*auth.AuthenticatedUser, error) {
	ctx = oidc.ClientContext(ctx, p.httpClient)

	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errNoIDToken
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	return &auth.AuthenticatedUser{
		Email:       claims.Email,
		Name:        claims.Name,
		Subject:     idToken.Subject,
		AccessToken: token.AccessToken,
		Provider:    p.name,
	}, nil
}

//nolint:ireturn
func (p *GiteaProvider) GetAccessChecker(token string) (auth.RepoAccessChecker, error) {
	return NewGiteaAccessChecker(p.forgeURL, token, p.httpClient), nil
}
