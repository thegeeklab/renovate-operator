package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v89/github"
)

const (
	defaultPageSize = 50
	httpTimeout     = 30 * time.Second
)

var (
	errInvalidRepoName = errors.New("invalid repository name format")
	errMissingAdmin    = errors.New("admin permissions required to manage webhooks")
)

type Provider struct {
	client   *github.Client
	forgeURL string
}

func NewProvider(ctx context.Context, endpoint, token string) (*Provider, error) {
	baseURL := sanitizeEndpoint(endpoint)
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	forgeURL := deriveForgeURL(endpoint)

	opts := []github.ClientOptionsFunc{
		github.WithAuthToken(token),
		github.WithTimeout(httpTimeout),
	}

	if baseURL != "https://api.github.com" {
		opts = append(opts, github.WithEnterpriseURLs(baseURL, baseURL))
	}

	client, err := github.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create github client: %w", err)
	}

	return &Provider{client: client, forgeURL: forgeURL}, nil
}

func (p *Provider) GetIdentity() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	user, _, err := p.client.Users.Get(ctx, "")
	if err != nil {
		return "", err
	}

	return user.GetLogin(), nil
}

func (p *Provider) EnsureWebhook(ctx context.Context, repoName, webhookURL, secret string) (string, error) {
	owner, repo, err := parseRepoName(repoName)
	if err != nil {
		return "", err
	}

	repoData, _, err := p.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to fetch repository: %w", err)
	}

	if !repoData.GetPermissions().GetAdmin() {
		return "", errMissingAdmin
	}

	desiredEvents := []string{"push", "pull_request", "issues"}

	var existingHook *github.Hook

	opts := &github.ListOptions{Page: 1, PerPage: defaultPageSize}

	for {
		hooks, resp, err := p.client.Repositories.ListHooks(ctx, owner, repo, opts)
		if err != nil {
			return "", fmt.Errorf("failed to list webhooks: %w", err)
		}

		for _, hook := range hooks {
			if hook.GetConfig().GetURL() == webhookURL {
				existingHook = hook

				break
			}
		}

		if existingHook != nil || resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	if existingHook != nil {
		editOpts := &github.Hook{
			Config: &github.HookConfig{
				URL:         new(webhookURL),
				ContentType: new("json"),
				Secret:      new(secret),
			},
			Events: desiredEvents,
			Active: new(true),
		}

		_, _, err := p.client.Repositories.EditHook(ctx, owner, repo, existingHook.GetID(), editOpts)
		if err != nil {
			return "", fmt.Errorf("failed to update existing webhook: %w", err)
		}

		return strconv.FormatInt(existingHook.GetID(), 10), nil
	}

	createOpts := &github.Hook{
		Config: &github.HookConfig{
			URL:         new(webhookURL),
			ContentType: new("json"),
			Secret:      new(secret),
		},
		Events: desiredEvents,
		Active: new(true),
	}

	newHook, _, err := p.client.Repositories.CreateHook(ctx, owner, repo, createOpts)
	if err != nil {
		return "", fmt.Errorf("failed to create webhook: %w", err)
	}

	return strconv.FormatInt(newHook.GetID(), 10), nil
}

func (p *Provider) DeleteWebhook(ctx context.Context, repoName, webhookID string) error {
	if webhookID == "" {
		return nil
	}

	owner, repo, err := parseRepoName(repoName)
	if err != nil {
		return err
	}

	id, err := strconv.ParseInt(webhookID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid webhook ID format: %w", err)
	}

	resp, err := p.client.Repositories.DeleteHook(ctx, owner, repo, id)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil
		}

		return fmt.Errorf("failed to delete webhook %s: %w", webhookID, err)
	}

	return nil
}

func (p *Provider) RepoURL(ctx context.Context, repoName string) (string, error) {
	owner, repo, err := parseRepoName(repoName)
	if err != nil {
		return "", err
	}

	repoData, _, err := p.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to fetch repository: %w", err)
	}

	if repoData.GetHTMLURL() != "" {
		return repoData.GetHTMLURL(), nil
	}

	return fmt.Sprintf("%s/%s/%s", p.forgeURL, owner, repo), nil
}

func sanitizeEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(endpoint, "/")

	if endpoint == "https://github.com" {
		return "https://api.github.com"
	}

	if !strings.HasSuffix(endpoint, "/api/v3") {
		endpoint += "/api/v3"
	}

	return endpoint
}

func deriveForgeURL(endpoint string) string {
	endpoint = strings.TrimRight(endpoint, "/")

	if endpoint == "" || endpoint == "https://github.com" || endpoint == "https://api.github.com" {
		return "https://github.com"
	}

	return strings.TrimSuffix(endpoint, "/api/v3")
}

func parseRepoName(fullRepo string) (string, string, error) {
	owner, repo, found := strings.Cut(fullRepo, "/")

	if !found || strings.Contains(repo, "/") {
		return "", "", fmt.Errorf("%w: %s", errInvalidRepoName, fullRepo)
	}

	return owner, repo, nil
}
