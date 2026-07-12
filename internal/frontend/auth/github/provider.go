package github

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v7"
	"github.com/google/go-github/v89/github"
	"github.com/thegeeklab/renovate-operator/internal/frontend/auth"
	"golang.org/x/oauth2"
	github_oauth "golang.org/x/oauth2/github"
)

var errNoRefreshToken = errors.New("no refresh_token in token response")

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

//nolint:tagliatelle // GitHub API uses snake_case
type githubRepo struct {
	FullName    string `json:"full_name"`
	Permissions struct {
		Push bool `json:"push"`
	} `json:"permissions"`
}

//nolint:tagliatelle // GitHub API uses snake_case
type githubUser struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

type GitHubProvider struct {
	name         string
	displayName  string
	iconURL      string
	endpoint     string
	forgeURL     string
	oauth2Config *oauth2.Config
	httpClient   *http.Client
}

func NewGitHubProvider(ctx context.Context, cfg auth.ProviderConfig) (*GitHubProvider, error) {
	httpClient := &http.Client{
		Timeout: defaultHTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.Insecure}, //nolint:gosec
		},
	}

	displayName := cfg.DisplayName
	if displayName == "" {
		displayName = "GitHub"
	}

	iconURL := cfg.IconURL
	if iconURL == "" {
		iconURL = "https://github.githubassets.com/favicons/favicon.svg"
	}

	endpoint := github_oauth.Endpoint
	if cfg.AuthURL != "" {
		endpoint.AuthURL = cfg.AuthURL
	}

	if cfg.Endpoint != "" && cfg.Endpoint != "https://github.com" {
		endpoint.AuthURL = strings.TrimRight(cfg.Endpoint, "/") + "/login/oauth/authorize"
		endpoint.TokenURL = strings.TrimRight(cfg.Endpoint, "/") + "/login/oauth/access_token"
	}

	return &GitHubProvider{
		name:        cfg.Name,
		displayName: displayName,
		iconURL:     iconURL,
		endpoint:    cfg.Endpoint,
		forgeURL:    cfg.ForgeURL,
		oauth2Config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     endpoint,
			Scopes:       []string{"read:user", "user:email", "repo"},
		},
		httpClient: httpClient,
	}, nil
}

func (p *GitHubProvider) Type() string {
	return auth.ProviderTypeGitHub
}

func (p *GitHubProvider) Name() string {
	return p.name
}

func (p *GitHubProvider) DisplayName() string {
	return p.displayName
}

func (p *GitHubProvider) IconURL() string {
	return p.iconURL
}

func (p *GitHubProvider) LoginURL(state string) string {
	return p.oauth2Config.AuthCodeURL(state)
}

func (p *GitHubProvider) HandleCallback(ctx context.Context, code string) (*auth.AuthenticatedUser, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)

	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

	return p.getUserFromToken(ctx, token)
}

func (p *GitHubProvider) RefreshToken(ctx context.Context, refreshToken string) (*auth.AuthenticatedUser, error) {
	if refreshToken == "" {
		return nil, errNoRefreshToken
	}

	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)

	tokenSource := p.oauth2Config.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})

	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return p.getUserFromToken(ctx, newToken)
}

func (p *GitHubProvider) ValidateToken(ctx context.Context, token string) (*auth.AuthenticatedUser, error) {
	if token == "" {
		return nil, auth.ErrInvalidToken
	}

	opts := []github.ClientOptionsFunc{
		github.WithAuthToken(token),
		github.WithTimeout(defaultHTTPTimeout),
	}

	apiURL := p.apiURL()
	if apiURL != "https://api.github.com" {
		opts = append(opts, github.WithEnterpriseURLs(apiURL, apiURL))
	}

	client, err := github.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create github client: %w", err)
	}

	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	email := user.GetEmail()
	if email == "" {
		email, _ = p.fetchPrimaryEmailWithToken(ctx, token)
	}

	return &auth.AuthenticatedUser{
		Email:       email,
		Name:        user.GetName(),
		Subject:     strconv.FormatInt(user.GetID(), 10),
		AvatarURL:   user.GetAvatarURL(),
		AccessToken: token,
		Provider:    p.name,
	}, nil
}

func (p *GitHubProvider) fetchPrimaryEmailWithToken(ctx context.Context, token string) (string, error) {
	apiURL := p.apiURL()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/user/emails", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)

		return "", fmt.Errorf("%w: %d", errUnexpectedStatus, resp.StatusCode)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("failed to decode emails: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	return "", nil
}

func (p *GitHubProvider) getUserFromToken(ctx context.Context, token *oauth2.Token) (*auth.AuthenticatedUser, error) {
	client := p.oauth2Config.Client(ctx, token)

	user, err := p.fetchUser(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	if user.Email == "" {
		user.Email, err = p.fetchPrimaryEmail(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch email: %w", err)
		}
	}

	return &auth.AuthenticatedUser{
		Email:        user.Email,
		Name:         user.Name,
		Subject:      strconv.FormatInt(user.ID, 10),
		AvatarURL:    user.AvatarURL,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenExpiry:  token.Expiry,
		Provider:     p.name,
	}, nil
}

func (p *GitHubProvider) fetchUser(ctx context.Context, client *http.Client) (*githubUser, error) {
	apiURL := p.apiURL()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)

		return nil, fmt.Errorf("%w: %d", errUnexpectedStatus, resp.StatusCode)
	}

	var user githubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return &user, nil
}

func (p *GitHubProvider) fetchPrimaryEmail(ctx context.Context, client *http.Client) (string, error) {
	apiURL := p.apiURL()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/user/emails", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)

		return "", fmt.Errorf("%w: %d", errUnexpectedStatus, resp.StatusCode)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("failed to decode emails: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	return "", nil
}

func (p *GitHubProvider) apiURL() string {
	if p.forgeURL != "" {
		return strings.TrimRight(p.forgeURL, "/")
	}

	if p.endpoint == "" || p.endpoint == "https://github.com" || p.endpoint == "https://api.github.com" {
		return "https://api.github.com"
	}

	return strings.TrimRight(p.endpoint, "/") + "/api/v3"
}

func (p *GitHubProvider) GetUserRepos(ctx context.Context, client *http.Client) (map[string]bool, error) {
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

func (p *GitHubProvider) IsUserRepo(ctx context.Context, client *http.Client, fullName string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, repoCheckTimeout)
	defer cancel()

	apiURL := p.apiURL()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/repos/%s", apiURL, fullName), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

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

	var repo githubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return false, fmt.Errorf("failed to decode repo: %w", err)
	}

	return repo.Permissions.Push, nil
}

func (p *GitHubProvider) fetchPageWithRetry(ctx context.Context, client *http.Client, page int) ([]githubRepo, error) {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = backoffInitial
	bo.MaxInterval = backoffMax
	bo.Multiplier = backoffMultiplier

	return backoff.Retry(ctx, func() ([]githubRepo, error) {
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
				return nil, backoff.RetryAfter(retryAfter, fmt.Errorf("%w: %d", errRateLimited, statusCode))
			}

			return nil, fmt.Errorf("%w: %d", errRateLimited, statusCode)
		}

		return nil, fmt.Errorf("%w: %d", errServerError, statusCode)
	}, backoff.WithBackOff(bo), backoff.WithMaxTries(backoffMaxTries))
}

func (p *GitHubProvider) fetchPage(ctx context.Context, client *http.Client, page int) (
	[]githubRepo, int, time.Duration, error,
) {
	apiURL := p.apiURL()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/user/repos?per_page=%d&page=%d", apiURL, defaultPageSize, page), nil)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch repos: %w", err)
	}

	defer resp.Body.Close()

	retryAfter := p.parseRetryAfter(resp)

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)

		return nil, resp.StatusCode, retryAfter, nil
	}

	var data []githubRepo
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, resp.StatusCode, 0, fmt.Errorf("failed to decode repos: %w", err)
	}

	return data, resp.StatusCode, 0, nil
}

func (p *GitHubProvider) parseRetryAfter(resp *http.Response) time.Duration {
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
