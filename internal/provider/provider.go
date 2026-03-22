package provider

import (
	"context"
	"errors"
)

var ErrNotImplemented = errors.New("provider not implemented")

// WebhookManager defines the interface for managing remote Git provider webhooks.
type WebhookManager interface {
	// EnsureWebhook creates a webhook if it doesn't exist and returns its ID.
	EnsureWebhook(ctx context.Context, repoName, webhookURL, secret string) (string, error)
	// DeleteWebhook removes the webhook from the remote provider.
	DeleteWebhook(ctx context.Context, repoName, webhookID string) error
}
