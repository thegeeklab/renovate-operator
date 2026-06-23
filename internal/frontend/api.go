package frontend

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/thegeeklab/renovate-operator/internal/frontend/auth"
	"github.com/thegeeklab/renovate-operator/internal/frontend/viewmodel"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// API response types for the web UI

type RenovatorInfo struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	UID       string    `json:"uid"`
	Schedule  string    `json:"schedule"`
	CreatedAt time.Time `json:"createdAt"`
}

// RenovatorDetails contains detailed information about a Renovator and its related resources.
type RenovatorDetails struct {
	RenovatorInfo
	GitRepos    []viewmodel.GitRepoInfo `json:"gitRepos"`
	Runners     []RunnerInfo            `json:"runners"`
	Discoveries []DiscoveryInfo         `json:"discoveries"`
}

type RunnerInfo struct {
	Name         string    `json:"name"`
	Namespace    string    `json:"namespace"`
	CreatedAt    time.Time `json:"createdAt"`
	RenovatorUID string    `json:"renovatorUid"`
}

type DiscoveryInfo struct {
	Name         string    `json:"name"`
	Namespace    string    `json:"namespace"`
	Schedule     string    `json:"schedule"`
	CreatedAt    time.Time `json:"createdAt"`
	RenovatorUID string    `json:"renovatorUid"`
}

// APIHandler manages the web UI API endpoints.
type APIHandler struct {
	client      client.Client
	dataFactory *DataFactory
}

// NewAPIHandler creates a new APIHandler.
func NewAPIHandler(client client.Client, clientset kubernetes.Interface, authManager *auth.Manager) *APIHandler {
	return &APIHandler{
		client:      client,
		dataFactory: NewDataFactory(client, clientset, authManager),
	}
}

// RegisterRoutes registers the API routes.
func (h *APIHandler) RegisterRoutes(router chi.Router) {
	router.Route("/api/v1", func(r chi.Router) {
		r.Get("/version", h.getVersion)
		r.Get("/renovators", h.getRenovators)
		r.Get("/gitrepos", h.getGitRepos)
		r.Get("/runners", h.getRunners)
		r.Get("/discoveries", h.getDiscoveries)
		r.Post("/discovery/start", h.startDiscovery)
		r.Get("/discovery/status", h.getDiscoveryStatus)
	})
}

func getOptionsFromRequest(r *http.Request) ListOptions {
	q := r.URL.Query()

	return ListOptions{
		Namespace: q.Get("namespace"),
		Renovator: q.Get("renovator"),
		SortBy:    q.Get("sort"),
		Order:     q.Get("order"),
		Search:    q.Get("search"),
	}
}

// getVersion returns the API version information.
func (h *APIHandler) getVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(struct {
		Version string `json:"version"`
	}{
		Version: "v1.0.0",
	}); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// getRenovators returns a list of all Renovator resources.
func (h *APIHandler) getRenovators(w http.ResponseWriter, r *http.Request) {
	opts := getOptionsFromRequest(r)

	result, err := h.dataFactory.GetRenovators(r.Context(), opts)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// getGitRepos returns a list of GitRepo resources with optional filtering.
func (h *APIHandler) getGitRepos(w http.ResponseWriter, r *http.Request) {
	opts := getOptionsFromRequest(r)

	result, err := h.dataFactory.GetGitRepos(r.Context(), opts)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)

		return
	}

	result = h.dataFactory.ApplyAccessFilter(r.Context(), result)

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func (h *APIHandler) getRunners(w http.ResponseWriter, r *http.Request) {
	opts := getOptionsFromRequest(r)

	result, err := h.dataFactory.GetRunners(r.Context(), opts)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// getDiscoveries returns a list of Discovery resources with optional filtering.
func (h *APIHandler) getDiscoveries(w http.ResponseWriter, r *http.Request) {
	opts := getOptionsFromRequest(r)

	result, err := h.dataFactory.GetDiscoveries(r.Context(), opts)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// startDiscovery triggers a discovery process.
func (h *APIHandler) startDiscovery(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement discovery start logic
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{
		Status:  "success",
		Message: "Discovery started",
	}); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// getDiscoveryStatus returns the current status of a discovery process.
func (h *APIHandler) getDiscoveryStatus(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement discovery status logic
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(struct {
		Status string `json:"status"`
	}{
		Status: "running",
	}); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
