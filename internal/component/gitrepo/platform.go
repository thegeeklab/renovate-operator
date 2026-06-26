package gitrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/thegeeklab/renovate-operator/internal/provider"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) reconcilePlatformInfo(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      r.renovate.Spec.Platform.Token.SecretKeyRef.Name,
		Namespace: r.instance.Namespace,
	}, secret); err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to get platform token secret: %w", err)
	}

	platformConfig := provider.PlatformConfig{
		Type:     string(r.renovate.Spec.Platform.Type),
		Endpoint: r.renovate.Spec.Platform.Endpoint,
		Token:    string(secret.Data[r.renovate.Spec.Platform.Token.SecretKeyRef.Key]),
	}

	providerManager, err := r.providerFactory(ctx, platformConfig)
	if err != nil {
		if errors.Is(err, provider.ErrNotImplemented) {
			log.V(1).Info("Provider not implemented, skipping platform info", "platform", r.renovate.Spec.Platform.Type)

			return &ctrl.Result{}, nil
		}

		log.Error(err, "Failed to initialize provider")

		return &ctrl.Result{}, err
	}

	repoURL, err := providerManager.RepoURL(ctx, r.instance.Spec.Name)
	if err != nil {
		log.Error(err, "Failed to get repository URL")

		return &ctrl.Result{}, err
	}

	platform := string(r.renovate.Spec.Platform.Type)

	if r.instance.Status.Platform == platform && r.instance.Status.RepoURL == repoURL {
		log.V(1).Info("Platform info already up to date")

		return &ctrl.Result{}, nil
	}

	log.Info("Updating platform info", "platform", platform, "repoURL", repoURL)

	patch := client.MergeFrom(r.instance.DeepCopy())
	r.instance.Status.Platform = platform
	r.instance.Status.RepoURL = repoURL

	if err := r.Status().Patch(ctx, r.instance, patch); err != nil && !api_errors.IsNotFound(err) {
		return &ctrl.Result{}, fmt.Errorf("failed to patch platform info in status: %w", err)
	}

	return &ctrl.Result{}, nil
}
