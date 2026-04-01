package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/provider"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) reconcileWebhook(ctx context.Context) (*ctrl.Result, error) {
	if !r.instance.DeletionTimestamp.IsZero() {
		return r.deleteWebhook(ctx)
	}

	return r.createWebhook(ctx)
}

func (r *Reconciler) createWebhook(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if r.externalURL == "" {
		log.V(1).Info("External URL is not configured, skipping webhook creation")

		return &ctrl.Result{}, nil
	}

	webhookManager, err := r.provider(ctx, r.Client, r.instance, r.renovate)
	if err != nil {
		if errors.Is(err, provider.ErrNotImplemented) {
			log.V(1).Info("Webhook management not implemented for this provider", "platform", r.renovate.Spec.Platform.Type)

			return &ctrl.Result{}, nil
		}

		log.Error(err, "Failed to initialize provider")

		return &ctrl.Result{}, err
	}

	secretName, err := k8s.DeterministicSubdomainName(r.instance.Name, "-webhook-secret")
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to generate webhook secret name: %w", err)
	}

	webhookSecret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: r.instance.Namespace}, webhookSecret); err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to get webhook secret: %w", err)
	}

	secretString := string(webhookSecret.Data[renovatev1beta1.WebhookSecretDataKey])

	baseURL := strings.TrimRight(r.externalURL, "/")
	webhookURL := fmt.Sprintf("%s/hooks/%s/%s", baseURL, r.instance.Namespace, r.instance.Name)

	webhookID, err := webhookManager.EnsureWebhook(ctx, r.instance.Spec.Name, webhookURL, secretString)
	if err != nil {
		log.Error(err, "Failed to ensure webhook")

		return &ctrl.Result{}, err
	}

	if r.instance.Status.WebhookID == webhookID {
		log.V(1).Info("Webhook already correctly configured", "webhookID", webhookID)

		return &ctrl.Result{}, nil
	}

	log.Info("Webhook ID changed or created, updating status", "oldID", r.instance.Status.WebhookID, "newID", webhookID)

	patch := client.MergeFrom(r.instance.DeepCopy())
	r.instance.Status.WebhookID = webhookID

	if err := r.Status().Patch(ctx, r.instance, patch); err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to patch webhook ID in status: %w", err)
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) deleteWebhook(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if r.instance.Status.WebhookID == "" {
		return &ctrl.Result{}, nil
	}

	webhookManager, err := r.provider(ctx, r.Client, r.instance, r.renovate)
	if err != nil {
		if !errors.Is(err, provider.ErrNotImplemented) {
			log.Error(err, "Failed to initialize provider for cleanup")

			return &ctrl.Result{}, err
		}

		log.V(1).Info("Webhook management not implemented for this provider, skipping cleanup")
	} else {
		if err := webhookManager.DeleteWebhook(ctx, r.instance.Spec.Name, r.instance.Status.WebhookID); err != nil {
			log.Error(err, "Failed to delete webhook from remote")

			return &ctrl.Result{}, err
		}

		log.Info("Successfully deleted webhook from remote")
	}

	patch := client.MergeFrom(r.instance.DeepCopy())
	r.instance.Status.WebhookID = ""

	if err := r.Status().Patch(ctx, r.instance, patch); err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to clear webhook ID in status: %w", err)
	}

	return &ctrl.Result{}, nil
}
