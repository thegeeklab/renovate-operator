package gitrepo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) reconcileWebhookSecret(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !r.instance.DeletionTimestamp.IsZero() {
		return &ctrl.Result{}, nil
	}

	secretName := fmt.Sprintf("%s-webhook-secret", r.instance.Name)
	webhookSecret := &corev1.Secret{}

	err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: r.instance.Namespace}, webhookSecret)
	if err == nil {
		return &ctrl.Result{}, nil
	}

	if !api_errors.IsNotFound(err) {
		return &ctrl.Result{}, fmt.Errorf("failed to fetch webhook secret: %w", err)
	}

	secretString, err := generateSecureToken()
	if err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to generate secure token: %w", err)
	}

	webhookSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: r.instance.Namespace,
		},
	}
	r.updateSecret(webhookSecret, secretString)

	if err := controllerutil.SetControllerReference(r.instance, webhookSecret, r.scheme); err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to set owner reference on secret: %w", err)
	}

	if err := r.Create(ctx, webhookSecret); err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to create webhook secret: %w", err)
	}

	log.V(1).Info("Generated and saved new webhook secret", "secretName", secretName)

	return &ctrl.Result{}, nil
}

// updateSecret configures the secret spec for the webhook.
func (r *Reconciler) updateSecret(secret *corev1.Secret, token string) {
	secret.Data = map[string][]byte{
		"secret": []byte(token),
	}
}

func generateSecureToken() (string, error) {
	const secureTokenLength = 32

	bytes := make([]byte, secureTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}
