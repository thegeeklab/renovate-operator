package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v7"
	"github.com/thegeeklab/renovate-operator/internal/frontend/auth"
	"golang.org/x/oauth2"
)

const (
	defaultWebURL      = "https://gitlab.com"
	defaultPageSize    = 50
	defaultHTTPTimeout = 30 * time.Second
	maxFetchTimeout    = 2 * time.Minute
	repoCheckTimeout   = 10 * time.Second
	maxRepoPages       = 200
	developerAccess    = 30

	backoffInitial    = 200 * time.Millisecond
	backoffMax        = 10 * time.Second
	backoffMultiplier = 2.0
	backoffMaxTries   = 3
)

var (
	errNoRefreshToken   = errors.New("no refresh_token in token response")
	errUnexpectedStatus = errors.New("unexpected status code")
	errServerError      = errors.New("server error")
	errRateLimited      = errors.New("rate limited")
	errInvalidURL       = errors.New("invalid GitLab URL")
	errMaxRepoPages     = errors.New("maximum project page limit reached")
)

// GitLabProvider implements GitLab OAuth, PAT validation, and project authorization.
type GitLabProvider struct {
	name         string
	displayName  string
	iconURL      string
	webURL       string
	apiURL       string
	oauth2Config *oauth2.Config
	httpClient   *http.Client
}

var _ auth.AuthProvider = (*GitLabProvider)(nil)

// NewGitLabProvider creates a GitLab.com or self-managed auth provider.
func NewGitLabProvider(ctx context.Context, cfg auth.ProviderConfig) (*GitLabProvider, error) {
	webURL, apiURL, err := deriveURLs(cfg.Endpoint, cfg.ForgeURL)
	if err != nil {
		return nil, err
	}

	displayName := cfg.DisplayName
	if displayName == "" {
		if webURL == defaultWebURL {
			displayName = "GitLab"
		} else {
			displayName = hostFromURL(webURL)
		}
	}

	iconURL := cfg.IconURL
	if iconURL == "" {
		iconURL = faviconURL(webURL)
	}

	authURL := webURL + "/oauth/authorize"
	if cfg.AuthURL != "" {
		authURL = cfg.AuthURL
	}

	httpClient := &http.Client{
		Timeout:   defaultHTTPTimeout,
		Transport: &http.Transport{TLSClientConfig: auth.NewTLSConfig(cfg.Insecure, cfg.CACert)},
	}

	return &GitLabProvider{
		name:        cfg.Name,
		displayName: displayName,
		iconURL:     iconURL,
		webURL:      webURL,
		apiURL:      apiURL,
		oauth2Config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authURL,
				TokenURL: webURL + "/oauth/token",
			},
			Scopes: []string{"openid", "profile", "email", "read_api"},
		},
		httpClient: httpClient,
	}, nil
}

func (p *GitLabProvider) Type() string        { return auth.ProviderTypeGitLab }
func (p *GitLabProvider) Name() string        { return p.name }
func (p *GitLabProvider) DisplayName() string { return p.displayName }
func (p *GitLabProvider) IconURL() string     { return p.iconURL }
func (p *GitLabProvider) LoginURL(state, verifier string) string {
	return p.oauth2Config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
}

func (p *GitLabProvider) HandleCallback(ctx context.Context, code, verifier string) (*auth.AuthenticatedUser, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)

	token, err := p.oauth2Config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

	return p.getUserFromToken(ctx, token)
}

func (p *GitLabProvider) RefreshToken(ctx context.Context, refreshToken string) (*auth.AuthenticatedUser, error) {
	if refreshToken == "" {
		return nil, errNoRefreshToken
	}

	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
	tokenSource := p.oauth2Config.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})

	token, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return p.getUserFromToken(ctx, token)
}

//nolint:tagliatelle // GitLab API uses snake_case.
type gitlabUser struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	PublicEmail string `json:"public_email"`
	AvatarURL   string `json:"avatar_url"`
}

func (p *GitLabProvider) getUserFromToken(
	ctx context.Context,
	token *oauth2.Token,
) (*auth.AuthenticatedUser, error) {
	user, err := p.fetchUser(ctx, p.oauth2Config.Client(ctx, token), "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	return p.authenticatedUser(user, token.AccessToken, token.RefreshToken, token.Expiry), nil
}

func (p *GitLabProvider) fetchUser(
	ctx context.Context,
	client *http.Client,
	privateToken string,
) (*gitlabUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiURL+"/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if privateToken != "" {
		req.Header.Set("PRIVATE-TOKEN", privateToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)

		return nil, fmt.Errorf("%w: %d", errUnexpectedStatus, resp.StatusCode)
	}

	user := &gitlabUser{}
	if err := json.NewDecoder(resp.Body).Decode(user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return user, nil
}

func (p *GitLabProvider) authenticatedUser(
	user *gitlabUser,
	accessToken, refreshToken string,
	expiry time.Time,
) *auth.AuthenticatedUser {
	email := user.Email
	if email == "" {
		email = user.PublicEmail
	}

	return &auth.AuthenticatedUser{
		Email:        email,
		Name:         user.Name,
		Subject:      strconv.FormatInt(user.ID, 10),
		AvatarURL:    user.AvatarURL,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenExpiry:  expiry,
		Provider:     p.name,
	}
}

func (p *GitLabProvider) ValidateToken(ctx context.Context, token string) (*auth.AuthenticatedUser, error) {
	if token == "" {
		return nil, auth.ErrInvalidToken
	}

	user, err := p.fetchUser(ctx, p.httpClient, token)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	return p.authenticatedUser(user, token, "", time.Time{}), nil
}

//nolint:tagliatelle // GitLab API uses snake_case.
type gitlabProject struct {
	PathWithNamespace string `json:"path_with_namespace"`
	Permissions       struct {
		ProjectAccess *gitlabAccess `json:"project_access"`
		GroupAccess   *gitlabAccess `json:"group_access"`
	} `json:"permissions"`
}

//nolint:tagliatelle // GitLab API uses snake_case.
type gitlabAccess struct {
	AccessLevel int `json:"access_level"`
}

func (p *GitLabProvider) GetUserRepos(ctx context.Context, client *http.Client) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(ctx, maxFetchTimeout)
	defer cancel()

	result := make(map[string]bool)
	completed := false

	for page := 1; page <= maxRepoPages; page++ {
		if err := ctx.Err(); err != nil {
			if len(result) > 0 {
				return result, fmt.Errorf("fetch projects cancelled with partial results: %w", err)
			}

			return result, fmt.Errorf("fetch projects cancelled: %w", err)
		}

		projects, err := p.fetchPageWithRetry(ctx, client, page)
		if err != nil {
			if len(result) > 0 {
				return result, fmt.Errorf("fetch failed with partial results: %w", err)
			}

			return result, err
		}

		for _, project := range projects {
			if project.PathWithNamespace != "" {
				result[project.PathWithNamespace] = true
			}
		}

		if len(projects) < defaultPageSize {
			completed = true

			break
		}
	}

	if !completed {
		return result, fmt.Errorf("fetch stopped with partial results: %w", errMaxRepoPages)
	}

	return result, nil
}

func (p *GitLabProvider) IsUserRepo(ctx context.Context, client *http.Client, fullName string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, repoCheckTimeout)
	defer cancel()

	projectID := url.PathEscape(fullName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiURL+"/projects/"+projectID, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check project access: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, resp.Body)

		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)

		return false, fmt.Errorf("%w: %d", errUnexpectedStatus, resp.StatusCode)
	}

	project := &gitlabProject{}
	if err := json.NewDecoder(resp.Body).Decode(project); err != nil {
		return false, fmt.Errorf("failed to decode project: %w", err)
	}

	return effectiveAccessLevel(project) >= developerAccess, nil
}

func effectiveAccessLevel(project *gitlabProject) int {
	level := 0
	if project.Permissions.ProjectAccess != nil {
		level = project.Permissions.ProjectAccess.AccessLevel
	}

	if project.Permissions.GroupAccess != nil && project.Permissions.GroupAccess.AccessLevel > level {
		level = project.Permissions.GroupAccess.AccessLevel
	}

	return level
}

func (p *GitLabProvider) fetchPageWithRetry(
	ctx context.Context,
	client *http.Client,
	page int,
) ([]gitlabProject, error) {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = backoffInitial
	bo.MaxInterval = backoffMax
	bo.Multiplier = backoffMultiplier

	return backoff.Retry(ctx, func() ([]gitlabProject, error) {
		projects, statusCode, retryAfter, err := p.fetchPage(ctx, client, page)
		if err != nil {
			return nil, err
		}

		if statusCode == http.StatusOK {
			return projects, nil
		}

		if statusCode != http.StatusTooManyRequests && statusCode < http.StatusInternalServerError {
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

func (p *GitLabProvider) fetchPage(
	ctx context.Context,
	client *http.Client,
	page int,
) ([]gitlabProject, int, time.Duration, error) {
	query := url.Values{
		"membership":                  []string{"true"},
		"min_access_level":            []string{strconv.Itoa(developerAccess)},
		"with_merge_requests_enabled": []string{"true"},
		"per_page":                    []string{strconv.Itoa(defaultPageSize)},
		"page":                        []string{strconv.Itoa(page)},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiURL+"/projects?"+query.Encode(), nil)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch projects: %w", err)
	}
	defer resp.Body.Close()

	retryAfter := parseRetryAfter(resp)
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)

		return nil, resp.StatusCode, retryAfter, nil
	}

	projects := []gitlabProject{}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, resp.StatusCode, 0, fmt.Errorf("failed to decode projects: %w", err)
	}

	return projects, resp.StatusCode, 0, nil
}

func parseRetryAfter(resp *http.Response) time.Duration {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	if retryAt, err := http.ParseTime(retryAfter); err == nil {
		return time.Until(retryAt)
	}

	return 0
}

func deriveURLs(endpoint, forgeURL string) (string, string, error) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	forgeURL = strings.TrimRight(strings.TrimSpace(forgeURL), "/")

	if endpoint == "" {
		endpoint = defaultWebURL
	}

	webURL := strings.TrimSuffix(endpoint, "/api/v4")
	apiURL := webURL + "/api/v4"

	if forgeURL != "" {
		if strings.HasSuffix(forgeURL, "/api/v4") {
			apiURL = forgeURL
		} else {
			apiURL = forgeURL + "/api/v4"
		}
	}

	for _, rawURL := range []string{webURL, apiURL} {
		parsed, err := url.ParseRequestURI(rawURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", "", fmt.Errorf("%w: %s", errInvalidURL, rawURL)
		}
	}

	return webURL, apiURL, nil
}

func hostFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	return parsed.Host
}

func faviconURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%s://%s/favicon.ico", parsed.Scheme, parsed.Host)
}
