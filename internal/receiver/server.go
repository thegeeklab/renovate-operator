package receiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/provider"
	"github.com/thegeeklab/renovate-operator/internal/receiver/gitea"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var receiverLog = logf.Log.WithName("receiver-server")

var (
	ErrPlatformTokenSecretNotConfigured = errors.New("platform token secret not configured")
	ErrTokenKeyNotFoundInSecret         = errors.New("token key not found in secret")
	ErrPlatformTokenSecretFetchFailed   = errors.New("failed to fetch platform token secret")
	ErrGitRepoInvalid                   = errors.New("GitRepo invalid")
	ErrRenovateConfigNotFound           = errors.New("RenovateConfig not found")
)

// ReceiverFactory is a function that creates a new Receiver for a specific platform.
type ReceiverFactory func() Receiver

var receiverFactories = map[renovatev1beta1.PlatformType]ReceiverFactory{
	renovatev1beta1.PlatformType_GITEA: func() Receiver { return gitea.NewReceiver() },
}

const (
	DefaultReadTimeout  = 10 * time.Second
	DefaultWriteTimeout = 30 * time.Second
	DefaultIdleTimeout  = 120 * time.Second

	// maxWebhookBodyBytes caps the webhook request body to protect against memory exhaustion.
	maxWebhookBodyBytes = 1 << 20 // 1 MiB
)

type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:         ":8083",
		ReadTimeout:  DefaultReadTimeout,
		WriteTimeout: DefaultWriteTimeout,
		IdleTimeout:  DefaultIdleTimeout,
	}
}

type Server struct {
	config   ServerConfig
	client   client.Client
	router   *mux.Router
	server   *http.Server
	provider provider.ProviderFactory
}

func NewServer(config ServerConfig, k8sClient client.Client) *Server {
	s := &Server{
		config:   config,
		client:   k8sClient,
		router:   mux.NewRouter(),
		provider: provider.DefaultProviderFactory,
	}

	s.router.HandleFunc("/hooks/{namespace}/{name}", s.handleIncomingWebhook).Methods("POST")

	s.server = &http.Server{
		Addr:         config.Addr,
		Handler:      s.router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return s
}

// Start runs the HTTP server and blocks until the context is closed or an error occurs.
func (s *Server) Start(ctx context.Context) error {
	const shutdownTimeout = 5 * time.Second

	receiverLog.Info("Starting Event Receiver server", "address", s.config.Addr)

	go func() {
		<-ctx.Done()
		receiverLog.Info("Shutting down Event Receiver server")

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()

		if err := s.server.Shutdown(shutdownCtx); err != nil {
			receiverLog.Error(err, "Event Receiver server shutdown error")
		}
	}()

	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		receiverLog.Error(err, "Event Receiver server error")

		return err
	}

	return nil
}

func (s *Server) handleIncomingWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]

	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)

		return
	}

	secretToken, err := s.resolveWebhookSecret(ctx, namespace, name)
	if err != nil {
		receiverLog.Error(err, "Failed to find webhook secret")
		http.Error(w, "Webhook secret not found", http.StatusNotFound)

		return
	}

	repo := &renovatev1beta1.GitRepo{}
	if err := s.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, repo); err != nil {
		http.Error(w, "GitRepo not found", http.StatusNotFound)

		return
	}

	config, err := s.resolveRenovateConfig(ctx, namespace, name, repo)
	if err != nil {
		receiverLog.Error(err, "Failed to resolve RenovateConfig for GitRepo")
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	factory, ok := receiverFactories[config.Spec.Platform.Type]
	if !ok {
		receiverLog.Info("Webhook verification not implemented for platform", "platform", config.Spec.Platform.Type)
		http.Error(w, "Platform verification not implemented", http.StatusNotImplemented)

		return
	}

	platformReceiver := factory()

	if err := platformReceiver.Validate(r, secretToken, body); err != nil {
		receiverLog.Error(err, "Webhook validation failed", "namespace", namespace, "name", name)
		http.Error(w, "Invalid signature", http.StatusForbidden)

		return
	}

	result, err := platformReceiver.Parse(r, body)
	if err != nil {
		receiverLog.Error(err, "Failed to parse webhook event")
		http.Error(w, "Bad Request", http.StatusBadRequest)

		return
	}

	if !result.ShouldTrigger {
		receiverLog.Info("Webhook processed, no trigger required", "namespace", namespace, "name", name)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))

		return
	}

	if result.RequireUserCheck {
		allowed, err := s.verifyWebhookUser(ctx, namespace, name, config, result.User)
		if err != nil {
			receiverLog.Error(err, "Failed to verify webhook user status")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

			return
		}

		if !allowed {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"accepted"}`))

			return
		}
	}

	patch := client.MergeFrom(repo.DeepCopy())

	if repo.Annotations == nil {
		repo.Annotations = make(map[string]string)
	}

	repo.Annotations[renovatev1beta1.RenovatorOperation] = renovatev1beta1.OperationRenovate

	if err := s.client.Patch(ctx, repo, patch); err != nil {
		receiverLog.Error(err, "Failed to apply trigger annotation")
		http.Error(w, "Failed to trigger run", http.StatusInternalServerError)

		return
	}

	receiverLog.Info("Renovate run triggered successfully", "namespace", namespace, "name", name)

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

func (s *Server) resolveWebhookSecret(ctx context.Context, namespace, name string) ([]byte, error) {
	secretName := fmt.Sprintf("%s-webhook-secret", name)
	webhookSecret := &corev1.Secret{}

	if err := s.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, webhookSecret); err != nil {
		return nil, err
	}

	return webhookSecret.Data[renovatev1beta1.WebhookSecretDataKey], nil
}

func (s *Server) resolveRenovateConfig(
	ctx context.Context,
	namespace, name string,
	repo *renovatev1beta1.GitRepo,
) (*renovatev1beta1.RenovateConfig, error) {
	renovatorID, ok := repo.Labels[renovatev1beta1.LabelRenovator]
	if !ok {
		receiverLog.Info("GitRepo is missing renovator label", "namespace", namespace, "name", name)

		return nil, ErrGitRepoInvalid
	}

	configList := &renovatev1beta1.RenovateConfigList{}
	if err := s.client.List(ctx, configList, client.InNamespace(namespace), client.MatchingLabels{
		renovatev1beta1.LabelRenovator: renovatorID,
	}); err != nil {
		return nil, err
	}

	if len(configList.Items) == 0 {
		receiverLog.Info("No RenovateConfig found for GitRepo", "namespace", namespace, "name", name)

		return nil, ErrRenovateConfigNotFound
	}

	return &configList.Items[0], nil
}

func (s *Server) verifyWebhookUser(
	ctx context.Context,
	namespace, name string,
	config *renovatev1beta1.RenovateConfig,
	webhookUser string,
) (bool, error) {
	platformToken, err := s.resolvePlatformToken(ctx, namespace, &config.Spec.Platform)
	if err != nil {
		receiverLog.Error(err, "Failed to resolve platform token", "namespace", namespace, "name", name)

		return false, err
	}

	providerManager, err := s.provider(
		ctx,
		provider.PlatformConfig{
			Type:     string(config.Spec.Platform.Type),
			Endpoint: config.Spec.Platform.Endpoint,
			Token:    platformToken,
		},
	)
	if err != nil {
		receiverLog.Error(err, "Failed to create platform provider", "namespace", namespace, "name", name)

		return false, err
	}

	allowedUser, err := providerManager.GetIdentity()
	if err != nil {
		receiverLog.Error(err, "Failed to get allowed user identity", "namespace", namespace, "name", name)

		return false, err
	}

	if webhookUser != allowedUser {
		receiverLog.Info("Webhook user does not match expected identity", "user", webhookUser)

		return false, nil
	}

	return true, nil
}

func (s *Server) resolvePlatformToken(
	ctx context.Context,
	namespace string,
	platform *renovatev1beta1.PlatformSpec,
) (string, error) {
	if platform.Token.SecretKeyRef == nil {
		return "", ErrPlatformTokenSecretNotConfigured
	}

	secret := &corev1.Secret{}
	if err := s.client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      platform.Token.SecretKeyRef.Name,
	}, secret); err != nil {
		return "", fmt.Errorf("%w: %w", ErrPlatformTokenSecretFetchFailed, err)
	}

	token, ok := secret.Data[platform.Token.SecretKeyRef.Key]
	if !ok {
		return "", ErrTokenKeyNotFoundInSecret
	}

	return string(token), nil
}
