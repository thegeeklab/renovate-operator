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
	"github.com/thegeeklab/renovate-operator/internal/receiver/gitea"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var receiverLog = logf.Log.WithName("receiver-server")

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
	config ServerConfig
	client client.Client
	router *mux.Router
	server *http.Server
}

func NewServer(config ServerConfig, k8sClient client.Client) *Server {
	s := &Server{
		config: config,
		client: k8sClient,
		router: mux.NewRouter(),
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

	secretName := fmt.Sprintf("%s-webhook-secret", name)

	webhookSecret := &corev1.Secret{}
	if err := s.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, webhookSecret); err != nil {
		receiverLog.Error(err, "Failed to find webhook secret")
		http.Error(w, "Webhook secret not found", http.StatusNotFound)

		return
	}

	secretToken := webhookSecret.Data[renovatev1beta1.WebhookSecretDataKey]

	repo := &renovatev1beta1.GitRepo{}
	if err := s.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, repo); err != nil {
		http.Error(w, "GitRepo not found", http.StatusNotFound)

		return
	}

	renovatorID, ok := repo.Labels[renovatev1beta1.LabelRenovator]
	if !ok {
		receiverLog.Info("GitRepo is missing renovator label", "namespace", namespace, "name", name)
		http.Error(w, "GitRepo invalid", http.StatusInternalServerError)

		return
	}

	configList := &renovatev1beta1.RenovateConfigList{}
	if err := s.client.List(ctx, configList, client.InNamespace(namespace), client.MatchingLabels{
		renovatev1beta1.LabelRenovator: renovatorID,
	}); err != nil {
		receiverLog.Error(err, "Failed to list RenovateConfigs")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	if len(configList.Items) == 0 {
		receiverLog.Info("No RenovateConfig found for GitRepo", "namespace", namespace, "name", name)
		http.Error(w, "RenovateConfig not found", http.StatusInternalServerError)

		return
	}

	config := &configList.Items[0]

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

	shouldTrigger, err := platformReceiver.Parse(r, body)
	if err != nil {
		receiverLog.Error(err, "Failed to parse webhook event")
		http.Error(w, "Bad Request", http.StatusBadRequest)

		return
	}

	if shouldTrigger {
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
	} else {
		receiverLog.Info("Webhook processed, no trigger required", "namespace", namespace, "name", name)
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}
