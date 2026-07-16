package factory

import (
	"context"
	"errors"

	"github.com/thegeeklab/renovate-operator/internal/provider"
	"github.com/thegeeklab/renovate-operator/internal/provider/gitea"
	"github.com/thegeeklab/renovate-operator/internal/provider/github"
	"github.com/thegeeklab/renovate-operator/internal/provider/gitlab"
)

var ErrNotImplemented = errors.New("provider not implemented")

type PlatformConfig struct {
	Type     string
	Endpoint string
	Token    string
}

type ProviderFactory func(
	ctx context.Context,
	config PlatformConfig,
) (provider.ProviderManager, error)

// DefaultProviderFactory is the default ProviderFactory implementation.
//
//nolint:ireturn
func DefaultProviderFactory(
	ctx context.Context, config PlatformConfig,
) (provider.ProviderManager, error) {
	switch config.Type {
	case "gitea":
		return gitea.NewProvider(ctx, config.Endpoint, config.Token)
	case "github":
		return github.NewProvider(ctx, config.Endpoint, config.Token)
	case "gitlab":
		return gitlab.NewProvider(ctx, config.Endpoint, config.Token)
	default:
		return nil, ErrNotImplemented
	}
}
