package gitea

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/thegeeklab/renovate-operator/internal/auth"
	"golang.org/x/oauth2"
)

var errNoIDToken = errors.New("no id_token in token response")

const (
	defaultPageSize    = 50
	maxFetchTimeout    = 2 * time.Minute
	repoCheckTimeout   = 10 * time.Second
	defaultHTTPTimeout = 30 * time.Second

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
	FullName string `json:"full_name"`
}

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
		Timeout: defaultHTTPTimeout,
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

	endpoint := provider.Endpoint()
	if cfg.AuthURL != "" {
		endpoint.AuthURL = cfg.AuthURL
	}

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

func (p *GiteaProvider) GetUserRepos(ctx context.Context, token string) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(ctx, maxFetchTimeout)
	defer cancel()

	result := make(map[string]bool)

	page := 1

	for {
		if err := ctx.Err(); err != nil {
			if len(result) > 0 {
				return result, fmt.Errorf("fetch repos cancelled with partial results: %w", err)
			}

			return result, fmt.Errorf("fetch repos cancelled: %w", err)
		}

		data, err := p.fetchPageWithRetry(ctx, page, token)
		if err != nil {
			if len(result) > 0 {
				return result, fmt.Errorf("fetch failed with partial results: %w", err)
			}

			return result, err
		}

		for _, repo := range data {
			result[repo.FullName] = true
		}

		if len(data) == 0 {
			break
		}

		page++
	}

	return result, nil
}

func (p *GiteaProvider) IsUserRepo(ctx context.Context, token, fullName string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, repoCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/api/v1/repos/%s", strings.TrimRight(p.forgeURL, "/"), fullName), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check repo access: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return false, nil
	}

	return false, fmt.Errorf("%w: %d", errUnexpectedStatus, resp.StatusCode)
}

func (p *GiteaProvider) fetchPageWithRetry(ctx context.Context, page int, token string) ([]giteaRepo, error) {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = backoffInitial
	bo.MaxInterval = backoffMax
	bo.Multiplier = backoffMultiplier

	return backoff.Retry(ctx, func() ([]giteaRepo, error) {
		data, statusCode, retryAfter, err := p.fetchPage(ctx, page, token)
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

func (p *GiteaProvider) fetchPage(ctx context.Context, page int, token string) (
	[]giteaRepo, int, time.Duration, error,
) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/api/v1/repos/search?limit=%d&page=%d",
			strings.TrimRight(p.forgeURL, "/"), defaultPageSize, page), nil)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch repos: %w", err)
	}

	defer resp.Body.Close()

	retryAfter := p.parseRetryAfter(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, retryAfter, nil
	}

	var respBody struct {
		OK   bool        `json:"ok"`
		Data []giteaRepo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, resp.StatusCode, 0, fmt.Errorf("failed to decode repos: %w", err)
	}

	return respBody.Data, resp.StatusCode, 0, nil
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
