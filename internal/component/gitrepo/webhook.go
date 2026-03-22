package gitrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/thegeeklab/renovate-operator/internal/provider"
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

	webhookManager, err := r.ProviderFactory(ctx, r.Client, r.instance, r.renovate)
	if err != nil {
		if errors.Is(err, provider.ErrNotImplemented) {
			log.V(1).Info("Webhook management not implemented for this provider", "platform", r.renovate.Spec.Platform.Type)

			return &ctrl.Result{}, nil
		}

		log.Error(err, "Failed to initialize provider")

		return &ctrl.Result{}, err
	}

	secretName := fmt.Sprintf("%s-webhook-secret", r.instance.Name)

	webhookSecret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: r.instance.Namespace}, webhookSecret); err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to get webhook secret: %w", err)
	}

	secretString := string(webhookSecret.Data["secret"])

	webhookID, err := webhookManager.EnsureWebhook(ctx, r.instance.Spec.Name, DummyWebhookURL, secretString)
	if err != nil {
		log.Error(err, "Failed to ensure webhook")

		return &ctrl.Result{}, err
	}

	if r.instance.Spec.WebhookID != webhookID {
		log.Info("Webhook ID changed or created, updating resource", "oldID", r.instance.Spec.WebhookID, "newID", webhookID)
		r.instance.Spec.WebhookID = webhookID

		if err := r.Update(ctx, r.instance); err != nil {
			return &ctrl.Result{}, err
		}

		log.Info("Successfully configured webhook", "webhookID", webhookID)
	} else {
		log.V(1).Info("Webhook already correctly configured", "webhookID", webhookID)
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) deleteWebhook(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if r.instance.Spec.WebhookID == "" {
		return &ctrl.Result{}, nil
	}

	webhookManager, err := r.ProviderFactory(ctx, r.Client, r.instance, r.renovate)
	if err != nil {
		if !errors.Is(err, provider.ErrNotImplemented) {
			log.Error(err, "Failed to initialize provider for cleanup")

			return &ctrl.Result{}, err
		}

		log.V(1).Info("Webhook management not implemented for this provider, skipping cleanup")
	} else {
		if err := webhookManager.DeleteWebhook(ctx, r.instance.Spec.Name, r.instance.Spec.WebhookID); err != nil {
			log.Error(err, "Failed to delete webhook from remote")

			return &ctrl.Result{}, err
		}

		log.Info("Successfully deleted webhook from remote")
	}

	r.instance.Spec.WebhookID = ""
	if err := r.Update(ctx, r.instance); err != nil {
		return &ctrl.Result{}, err
	}

	return &ctrl.Result{}, nil
}
