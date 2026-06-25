package gitea

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v6"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/thegeeklab/renovate-operator/internal/frontend/auth"
	"golang.org/x/oauth2"
)

var (
	errNoIDToken      = errors.New("no id_token in token response")
	errNoRefreshToken = errors.New("no refresh_token in token response")
)

const (
	defaultPageSize    = 50
	maxFetchTimeout    = 2 * time.Minute
	repoCheckTimeout   = 10 * time.Second
	defaultHTTPTimeout = 30 * time.Second

	maxRepoPages = 200

	backoffInitial    = 200 * time.Millisecond
	backoffMax        = 10 * time.Second
	backoffMultiplier = 2.0
	backoffMaxTries   = 3
)

var (
	errUnexpectedStatus = errors.New("unexpected status code")
	errServerError      = errors.New("server error")
	errRateLimited      = errors.New("rate limited")
)

//nolint:tagliatelle // Gitea API uses snake_case
type giteaRepo struct {
	FullName    string `json:"full_name"`
	Permissions struct {
		Push bool `json:"push"`
	} `json:"permissions"`
}

type GiteaProvider struct {
	name         string
	displayName  string
	iconURL      string
	endpoint     string
	clientID     string
	clientSecret string
	redirectURL  string
	forgeURL     string
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	httpClient   *http.Client
}

func NewGiteaProvider(ctx context.Context, cfg auth.ProviderConfig) (*GiteaProvider, error) {
	httpClient := &http.Client{
		Timeout: defaultHTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.Insecure}, //nolint:gosec
		},
	}

	ctx = oidc.ClientContext(ctx, httpClient)

	provider, err := oidc.NewProvider(ctx, cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OIDC provider: %w", err)
	}

	oidcConfig := &oidc.Config{ClientID: cfg.ClientID}

	endpoint := provider.Endpoint()
	endpoint.AuthURL = resolveAuthURL(cfg.AuthURL, cfg.Endpoint)

	displayName := cfg.DisplayName
	if displayName == "" {
		displayName = hostFromURL(resolveAuthURL(cfg.AuthURL, cfg.Endpoint))
	}

	iconURL := cfg.IconURL
	if iconURL == "" {
		iconURL = faviconURL(resolveAuthURL(cfg.AuthURL, cfg.Endpoint))
	}

	return &GiteaProvider{
		name:         cfg.Name,
		displayName:  displayName,
		iconURL:      iconURL,
		endpoint:     cfg.Endpoint,
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
			Endpoint:     endpoint,
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

func (p *GiteaProvider) DisplayName() string {
	return p.displayName
}

func (p *GiteaProvider) IconURL() string {
	return p.iconURL
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

	return p.getUserFromToken(ctx, token)
}

func (p *GiteaProvider) RefreshToken(ctx context.Context, refreshToken string) (*auth.AuthenticatedUser, error) {
	if refreshToken == "" {
		return nil, errNoRefreshToken
	}

	ctx = oidc.ClientContext(ctx, p.httpClient)

	tokenSource := p.oauth2Config.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})

	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return p.getUserFromToken(ctx, newToken)
}

func (p *GiteaProvider) getUserFromToken(ctx context.Context, token *oauth2.Token) (*auth.AuthenticatedUser, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errNoIDToken
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	var claims struct {
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	return &auth.AuthenticatedUser{
		Email:        claims.Email,
		Name:         claims.Name,
		Subject:      idToken.Subject,
		AvatarURL:    claims.Picture,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenExpiry:  token.Expiry,
		Provider:     p.name,
	}, nil
}

func (p *GiteaProvider) GetUserRepos(ctx context.Context, client *http.Client) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(ctx, maxFetchTimeout)
	defer cancel()

	result := make(map[string]bool)

	for page := 1; page <= maxRepoPages; page++ {
		if err := ctx.Err(); err != nil {
			if len(result) > 0 {
				return result, fmt.Errorf("fetch repos cancelled with partial results: %w", err)
			}

			return result, fmt.Errorf("fetch repos cancelled: %w", err)
		}

		data, err := p.fetchPageWithRetry(ctx, client, page)
		if err != nil {
			if len(result) > 0 {
				return result, fmt.Errorf("fetch failed with partial results: %w", err)
			}

			return result, err
		}

		for _, repo := range data {
			if repo.Permissions.Push {
				result[repo.FullName] = true
			}
		}

		if len(data) < defaultPageSize {
			break
		}
	}

	return result, nil
}

func (p *GiteaProvider) IsUserRepo(ctx context.Context, client *http.Client, fullName string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, repoCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/api/v1/repos/%s", strings.TrimRight(p.forgeURL, "/"), fullName), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check repo access: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		_, _ = io.Copy(io.Discard, resp.Body)

		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)

		return false, fmt.Errorf("%w: %d", errUnexpectedStatus, resp.StatusCode)
	}

	var repo giteaRepo
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return false, fmt.Errorf("failed to decode repo: %w", err)
	}

	return repo.Permissions.Push, nil
}

func (p *GiteaProvider) fetchPageWithRetry(ctx context.Context, client *http.Client, page int) ([]giteaRepo, error) {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = backoffInitial
	bo.MaxInterval = backoffMax
	bo.Multiplier = backoffMultiplier

	return backoff.Retry(ctx, func() ([]giteaRepo, error) {
		data, statusCode, retryAfter, err := p.fetchPage(ctx, client, page)
		if err != nil {
			return nil, err
		}

		if statusCode == http.StatusOK {
			return data, nil
		}

		if statusCode != http.StatusTooManyRequests && statusCode < 500 {
			return nil, backoff.Permanent(fmt.Errorf("%w: %d", errUnexpectedStatus, statusCode))
		}

		if statusCode == http.StatusTooManyRequests {
			if retryAfter > 0 {
				return nil, backoff.RetryAfter(int(retryAfter.Seconds()))
			}

			return nil, fmt.Errorf("%w: %d", errRateLimited, statusCode)
		}

		return nil, fmt.Errorf("%w: %d", errServerError, statusCode)
	}, backoff.WithBackOff(bo), backoff.WithMaxTries(backoffMaxTries))
}

func (p *GiteaProvider) fetchPage(ctx context.Context, client *http.Client, page int) (
	[]giteaRepo, int, time.Duration, error,
) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/api/v1/user/repos?limit=%d&page=%d",
			strings.TrimRight(p.forgeURL, "/"), defaultPageSize, page), nil)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch repos: %w", err)
	}

	defer resp.Body.Close()

	retryAfter := p.parseRetryAfter(resp)

	if resp.StatusCode != http.StatusOK {
		// Drain the body so the underlying connection can be reused (keep-alive).
		// This matters because non-200 responses (429/5xx) are retried with backoff.
		_, _ = io.Copy(io.Discard, resp.Body)

		return nil, resp.StatusCode, retryAfter, nil
	}

	var data []giteaRepo
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, resp.StatusCode, 0, fmt.Errorf("failed to decode repos: %w", err)
	}

	return data, resp.StatusCode, 0, nil
}

func (p *GiteaProvider) parseRetryAfter(resp *http.Response) time.Duration {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	if t, err := http.ParseTime(retryAfter); err == nil {
		return time.Until(t)
	}

	return 0
}

func hostFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	return u.Host
}

func faviconURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%s://%s/favicon.ico", u.Scheme, u.Host)
}

func resolveAuthURL(override, issuerURL string) string {
	if override != "" {
		return override
	}

	return strings.TrimRight(issuerURL, "/") + "/login/oauth/authorize"
}
