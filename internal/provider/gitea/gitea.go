package gitea

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/sdk/gitea"
)

const defaultPageSize = 50

var (
	errInvalidRepoName = errors.New("invalid repository name format")
	errMissingAdmin    = errors.New("admin permissions required to manage webhooks")
)

type Provider struct {
	client *gitea.Client
}

// NewProvider initializes a new Gitea provider.
// Note: The context passed here is bound to the client instance by the Gitea SDK.
// It is important that a new Provider (and thus a new gitea.Client) is instantiated
// for every reconciliation loop. Do not cache the Provider or Client, as subsequent
// reconciles would fail with "context canceled" errors.
func NewProvider(ctx context.Context, endpoint, token string) (*Provider, error) {
	cleanEndpoint := sanitizeEndpoint(endpoint)

	client, err := gitea.NewClient(cleanEndpoint, gitea.SetContext(ctx), gitea.SetToken(token))
	if err != nil {
		return nil, fmt.Errorf("failed to create gitea client: %w", err)
	}

	return &Provider{client: client}, nil
}

func (p *Provider) EnsureWebhook(ctx context.Context, repoName, webhookURL, secret string) (string, error) {
	owner, repo, err := parseRepoName(repoName)
	if err != nil {
		return "", err
	}

	repoData, _, err := p.client.GetRepo(owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to fetch repository: %w", err)
	}

	if repoData.Permissions == nil || !repoData.Permissions.Admin {
		return "", errMissingAdmin
	}

	desiredEvents := []string{"push", "pull_request"}

	var existingHook *gitea.Hook

	opts := gitea.ListHooksOptions{ListOptions: gitea.ListOptions{Page: 1, PageSize: defaultPageSize}}

	for {
		hooks, resp, err := p.client.ListRepoHooks(owner, repo, opts)
		if err != nil {
			return "", fmt.Errorf("failed to list webhooks: %w", err)
		}

		for _, hook := range hooks {
			if hook.Config["url"] == webhookURL {
				currentHook := hook
				existingHook = currentHook

				break
			}
		}

		if existingHook != nil || resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	if existingHook != nil {
		editOpts := gitea.EditHookOption{
			Config: map[string]string{
				"url":          webhookURL,
				"content_type": "json",
				"secret":       secret,
			},
			Events: desiredEvents,
			Active: new(true),
		}

		_, err := p.client.EditRepoHook(owner, repo, existingHook.ID, editOpts)
		if err != nil {
			return "", fmt.Errorf("failed to update existing webhook: %w", err)
		}

		return strconv.FormatInt(existingHook.ID, 10), nil
	}

	createOpts := gitea.CreateHookOption{
		Type: "gitea",
		Config: map[string]string{
			"url":          webhookURL,
			"content_type": "json",
			"secret":       secret,
		},
		Events: desiredEvents,
		Active: true,
	}

	newHook, _, err := p.client.CreateRepoHook(owner, repo, createOpts)
	if err != nil {
		return "", fmt.Errorf("failed to create webhook: %w", err)
	}

	return strconv.FormatInt(newHook.ID, 10), nil
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

	resp, err := p.client.DeleteRepoHook(owner, repo, id)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil
		}

		return fmt.Errorf("failed to delete webhook %s: %w", webhookID, err)
	}

	return nil
}

// sanitizeEndpoint removes trailing slashes and the API suffix
// because the Gitea SDK automatically appends /api/v1 internally.
func sanitizeEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(endpoint, "/")

	return strings.TrimSuffix(endpoint, "/api/v1")
}

func parseRepoName(fullRepo string) (string, string, error) {
	owner, repo, found := strings.Cut(fullRepo, "/")

	if !found || strings.Contains(repo, "/") {
		return "", "", fmt.Errorf("%w: %s", errInvalidRepoName, fullRepo)
	}

	return owner, repo, nil
}
