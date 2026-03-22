package receiver

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/receiver/gitea"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TriggerAnnotation = "renovate.thegeeklab.de/trigger-run"

type Handler struct {
	client client.Client
}

func NewHandler(client client.Client) *Handler {
	return &Handler{
		client: client,
	}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/hooks/{namespace}/{name}", h.handleIncomingWebhook).Methods("POST")
}

func (h *Handler) handleIncomingWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)

		return
	}
	defer r.Body.Close()

	// 1. Fetch Webhook Secret
	secretName := fmt.Sprintf("%s-webhook-secret", name)

	webhookSecret := &corev1.Secret{}
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, webhookSecret); err != nil {
		receiverLog.Error(err, "Failed to find webhook secret")
		http.Error(w, "Webhook secret not found", http.StatusNotFound)

		return
	}

	secretToken := webhookSecret.Data["secret"]

	// 2. Fetch GitRepo
	repo := &renovatev1beta1.GitRepo{}
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, repo); err != nil {
		http.Error(w, "GitRepo not found", http.StatusNotFound)

		return
	}

	renovatorID, ok := repo.Labels[renovatev1beta1.LabelRenovator]
	if !ok {
		receiverLog.Info("GitRepo is missing renovator label", "namespace", namespace, "name", name)
		http.Error(w, "GitRepo invalid", http.StatusInternalServerError)

		return
	}

	// 3. Find matching RenovateConfig via label
	configList := &renovatev1beta1.RenovateConfigList{}
	if err := h.client.List(ctx, configList, client.InNamespace(namespace), client.MatchingLabels{
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

	// 4. Route to the correct Processor
	var processor Processor

	switch config.Spec.Platform.Type {
	case "gitea":
		processor = gitea.NewProcessor()
	default:
		receiverLog.Info("Webhook verification not implemented for platform", "platform", config.Spec.Platform.Type)
		http.Error(w, "Platform verification not implemented", http.StatusNotImplemented)

		return
	}

	// 5. Validate Authenticity
	if err := processor.Validate(r, secretToken, body); err != nil {
		receiverLog.Error(err, "Webhook validation failed", "namespace", namespace, "name", name)
		http.Error(w, "Invalid signature", http.StatusForbidden)

		return
	}

	// 6. Parse Event
	shouldTrigger, err := processor.Parse(r, body)
	if err != nil {
		receiverLog.Error(err, "Failed to parse webhook event")
		http.Error(w, "Bad Request", http.StatusBadRequest)

		return
	}

	// 7. Trigger Run
	if shouldTrigger {
		if repo.Annotations == nil {
			repo.Annotations = make(map[string]string)
		}

		repo.Annotations[TriggerAnnotation] = strconv.FormatInt(time.Now().UnixNano(), 10)

		if err := h.client.Update(ctx, repo); err != nil {
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
