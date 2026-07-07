package provider

import "context"

// ListReposOptions are platform-agnostic options for ListRepos.
type ListReposOptions struct {
	// SkipForks, when true, excludes forked repositories from the result.
	SkipForks bool
}

// Repo is the platform-agnostic representation of a repository returned by ListRepos.
type Repo struct {
	// Name is the full repository name in "owner/repo" format.
	Name string
	// IsFork reports whether the repository is a fork of another repository.
	IsFork bool
}

// ProviderManager defines the interface for interacting with a remote Git provider:
// managing repository webhooks and resolving the identity associated with the configured token.
type ProviderManager interface {
	// GetIdentity returns the identity of the user associated with the provided token.
	GetIdentity() (string, error)
	// EnsureWebhook creates a webhook if it doesn't exist and returns its ID.
	EnsureWebhook(ctx context.Context, repoName, webhookURL, secret string) (string, error)
	// DeleteWebhook removes the webhook from the remote provider.
	DeleteWebhook(ctx context.Context, repoName, webhookID string) error
	// RepoURL returns the web-accessible URL for a repository.
	RepoURL(ctx context.Context, repoName string) (string, error)
	// ListRepos returns repositories visible to the authenticated identity,
	// applying the given options. Results are paginated internally.
	ListRepos(ctx context.Context, opts ListReposOptions) ([]Repo, error)
}
