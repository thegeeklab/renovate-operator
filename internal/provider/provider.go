package provider

import (
	"context"
	"errors"

	"github.com/thegeeklab/renovate-operator/internal/provider/gitea"
)

var ErrNotImplemented = errors.New("provider not implemented")

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
}

type PlatformConfig struct {
	Type     string
	Endpoint string
	Token    string
}

type ProviderFactory func(
	ctx context.Context,
	config PlatformConfig,
) (ProviderManager, error)

// DefaultProviderFactory is the default ProviderFactory implementation.
//
//nolint:ireturn
func DefaultProviderFactory(
	ctx context.Context, config PlatformConfig,
) (ProviderManager, error) {
	switch config.Type {
	case "gitea":
		return gitea.NewProvider(ctx, config.Endpoint, config.Token)
	default:
		return nil, ErrNotImplemented
	}
}
