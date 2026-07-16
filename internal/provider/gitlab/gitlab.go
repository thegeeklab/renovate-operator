package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"

	"github.com/thegeeklab/renovate-operator/internal/provider"
	"github.com/thegeeklab/renovate-operator/pkg/util"
)

const (
	defaultAPIURL          = "https://gitlab.com/api/v4/"
	defaultPageSize        = 50
	minProjectPathSegments = 2
	httpTimeout            = 30 * time.Second
)

var (
	errInvalidProjectName = errors.New("invalid project name format")
	errMissingMaintainer  = errors.New("maintainer permissions required to manage project hooks")
)

// Provider manages GitLab projects and project hooks.
type Provider struct {
	client   *gitlabapi.Client
	forgeURL string
}

var _ provider.ProviderManager = (*Provider)(nil)

// NewProvider creates a GitLab provider for GitLab.com or a self-managed instance.
func NewProvider(ctx context.Context, endpoint, token string) (*Provider, error) {
	apiURL, forgeURL := normalizeEndpoint(endpoint)
	httpClient := &http.Client{Timeout: httpTimeout}

	client, err := gitlabapi.NewClient(
		token,
		gitlabapi.WithBaseURL(apiURL),
		gitlabapi.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	return &Provider{client: client, forgeURL: forgeURL}, nil
}

func (p *Provider) GetIdentity() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	user, _, err := p.client.Users.CurrentUser(gitlabapi.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to fetch current user: %w", err)
	}

	return user.Username, nil
}

func (p *Provider) EnsureWebhook(ctx context.Context, repoName, webhookURL, secret string) (string, error) {
	projectPath, err := parseProjectPath(repoName)
	if err != nil {
		return "", err
	}

	project, _, err := p.client.Projects.GetProject(projectPath, nil, gitlabapi.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to fetch project: %w", err)
	}

	if effectiveAccessLevel(project.Permissions) < gitlabapi.MaintainerPermissions {
		return "", errMissingMaintainer
	}

	listOpts := &gitlabapi.ListProjectHooksOptions{
		ListOptions: gitlabapi.ListOptions{Page: 1, PerPage: defaultPageSize},
	}

	var existingHook *gitlabapi.ProjectHook

	for {
		hooks, resp, err := p.client.Projects.ListProjectHooks(
			projectPath,
			listOpts,
			gitlabapi.WithContext(ctx),
		)
		if err != nil {
			return "", fmt.Errorf("failed to list project hooks: %w", err)
		}

		for _, hook := range hooks {
			if hook.URL == webhookURL {
				existingHook = hook

				break
			}
		}

		if existingHook != nil || resp.NextPage == 0 {
			break
		}

		listOpts.Page = resp.NextPage
	}

	if existingHook != nil {
		editOpts := &gitlabapi.EditProjectHookOptions{
			URL:                   new(webhookURL),
			Token:                 new(secret),
			PushEvents:            new(true),
			MergeRequestsEvents:   new(true),
			IssuesEvents:          new(true),
			EnableSSLVerification: new(true),
		}

		_, _, err = p.client.Projects.EditProjectHook(
			projectPath,
			existingHook.ID,
			editOpts,
			gitlabapi.WithContext(ctx),
		)
		if err != nil {
			return "", fmt.Errorf("failed to update existing project hook: %w", err)
		}

		return strconv.FormatInt(existingHook.ID, 10), nil
	}

	createOpts := &gitlabapi.AddProjectHookOptions{
		URL:                   new(webhookURL),
		Token:                 new(secret),
		PushEvents:            new(true),
		MergeRequestsEvents:   new(true),
		IssuesEvents:          new(true),
		EnableSSLVerification: new(true),
	}

	hook, _, err := p.client.Projects.AddProjectHook(
		projectPath,
		createOpts,
		gitlabapi.WithContext(ctx),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create project hook: %w", err)
	}

	return strconv.FormatInt(hook.ID, 10), nil
}

func (p *Provider) DeleteWebhook(ctx context.Context, repoName, webhookID string) error {
	if webhookID == "" {
		return nil
	}

	projectPath, err := parseProjectPath(repoName)
	if err != nil {
		return err
	}

	hookID, err := strconv.ParseInt(webhookID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid webhook ID format: %w", err)
	}

	resp, err := p.client.Projects.DeleteProjectHook(
		projectPath,
		hookID,
		gitlabapi.WithContext(ctx),
	)
	if err != nil {
		var responseErr *gitlabapi.ErrorResponse
		if (resp != nil && resp.StatusCode == http.StatusNotFound) ||
			(errors.As(err, &responseErr) && responseErr.HasStatusCode(http.StatusNotFound)) {
			return nil
		}

		return fmt.Errorf("failed to delete project hook %s: %w", webhookID, err)
	}

	return nil
}

func (p *Provider) RepoURL(ctx context.Context, repoName string) (string, error) {
	projectPath, err := parseProjectPath(repoName)
	if err != nil {
		return "", err
	}

	project, _, err := p.client.Projects.GetProject(
		projectPath,
		nil,
		gitlabapi.WithContext(ctx),
	)
	if err != nil {
		return "", fmt.Errorf("failed to fetch project: %w", err)
	}

	if project.WebURL != "" {
		return project.WebURL, nil
	}

	return p.forgeURL + "/" + projectPath, nil
}

// ListRepos returns GitLab member projects with Developer-or-higher access
// and merge requests enabled, applying portable filters locally.
func (p *Provider) ListRepos(ctx context.Context, opts provider.ListReposOptions) ([]provider.Repo, error) {
	listOpts := &gitlabapi.ListProjectsOptions{
		ListOptions:              gitlabapi.ListOptions{Page: 1, PerPage: defaultPageSize},
		Membership:               new(true),
		MinAccessLevel:           new(gitlabapi.DeveloperPermissions),
		WithMergeRequestsEnabled: new(true),
	}

	var repos []provider.Repo

	for {
		projects, resp, err := p.client.Projects.ListProjects(
			listOpts,
			gitlabapi.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list projects: %w", err)
		}

		for _, project := range projects {
			if project.PathWithNamespace == "" {
				continue
			}

			isFork := project.ForkedFromProject != nil
			if opts.SkipForks && isFork {
				continue
			}

			if len(opts.Topics) > 0 && !util.ContainsAll(project.Topics, opts.Topics) {
				continue
			}

			repos = append(repos, provider.Repo{
				Name:   project.PathWithNamespace,
				IsFork: isFork,
			})
		}

		if resp.NextPage == 0 {
			break
		}

		listOpts.Page = resp.NextPage
	}

	return repos, nil
}

func effectiveAccessLevel(permissions *gitlabapi.Permissions) gitlabapi.AccessLevelValue {
	if permissions == nil {
		return gitlabapi.NoPermissions
	}

	level := gitlabapi.NoPermissions
	if permissions.ProjectAccess != nil {
		level = permissions.ProjectAccess.AccessLevel
	}

	if permissions.GroupAccess != nil && permissions.GroupAccess.AccessLevel > level {
		level = permissions.GroupAccess.AccessLevel
	}

	return level
}

func parseProjectPath(projectPath string) (string, error) {
	projectPath = strings.TrimSpace(projectPath)
	segments := strings.Split(projectPath, "/")

	if len(segments) < minProjectPathSegments {
		return "", fmt.Errorf("%w: %s", errInvalidProjectName, projectPath)
	}

	if slices.Contains(segments, "") {
		return "", fmt.Errorf("%w: %s", errInvalidProjectName, projectPath)
	}

	return projectPath, nil
}

func normalizeEndpoint(endpoint string) (string, string) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")

	if endpoint == "" || endpoint == "https://gitlab.com" || endpoint == "https://gitlab.com/api/v4" {
		return defaultAPIURL, "https://gitlab.com"
	}

	forgeURL := strings.TrimSuffix(endpoint, "/api/v4")

	return forgeURL + "/api/v4/", forgeURL
}
