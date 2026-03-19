package frontend

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/thegeeklab/renovate-operator/internal/logstore"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// API response types for the web UI

type RenovatorInfo struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	UID       string    `json:"uid"`
	Schedule  string    `json:"schedule"`
	Ready     bool      `json:"ready"`
	CreatedAt time.Time `json:"createdAt"`
}

// RenovatorDetails contains detailed information about a Renovator and its related resources.
type RenovatorDetails struct {
	RenovatorInfo
	GitRepos    []GitRepoInfo   `json:"gitRepos"`
	Runners     []RunnerInfo    `json:"runners"`
	Discoveries []DiscoveryInfo `json:"discoveries"`
}

type GitRepoInfo struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	WebhookID string    `json:"webhookId"`
	Ready     bool      `json:"ready"`
	CreatedAt time.Time `json:"createdAt"`
}

type RunnerInfo struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Ready     bool      `json:"ready"`
	CreatedAt time.Time `json:"createdAt"`
}

type DiscoveryInfo struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Schedule  string    `json:"schedule"`
	Ready     bool      `json:"ready"`
	CreatedAt time.Time `json:"createdAt"`
}

type JobInfo struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Runner    string    `json:"runner"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// APIHandler manages the web UI API endpoints.
type APIHandler struct {
	client      client.Client
	dataFactory *DataFactory
	logManager  *logstore.Manager
}

// NewAPIHandler creates a new APIHandler.
func NewAPIHandler(client client.Client, logManager *logstore.Manager) *APIHandler {
	return &APIHandler{
		client:      client,
		dataFactory: NewDataFactory(client),
		logManager:  logManager,
	}
}

// RegisterRoutes registers the API routes.
func (h *APIHandler) RegisterRoutes(router *mux.Router) {
	apiV1 := router.PathPrefix("/api/v1").Subrouter()
	apiV1.HandleFunc("/version", h.getVersion).Methods("GET")
	apiV1.HandleFunc("/renovators", h.getRenovators).Methods("GET")
	apiV1.HandleFunc("/gitrepos", h.getGitRepos).Methods("GET")
	apiV1.HandleFunc("/runners", h.getRunners).Methods("GET")
	apiV1.HandleFunc("/discoveries", h.getDiscoveries).Methods("GET")
	apiV1.HandleFunc("/discovery/start", h.startDiscovery).Methods("POST")
	apiV1.HandleFunc("/discovery/status", h.getDiscoveryStatus).Methods("GET")
}

func getOptionsFromRequest(r *http.Request) ListOptions {
	q := r.URL.Query()

	return ListOptions{
		Namespace: q.Get("namespace"),
		Renovator: q.Get("renovator"),
		SortBy:    q.Get("sort"),
		Order:     q.Get("order"),
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// getRenovators returns a list of all Renovator resources.
func (h *APIHandler) getRenovators(w http.ResponseWriter, r *http.Request) {
	opts := getOptionsFromRequest(r)

	result, err := h.dataFactory.GetRenovators(r.Context(), opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// getGitRepos returns a list of GitRepo resources with optional filtering.
func (h *APIHandler) getGitRepos(w http.ResponseWriter, r *http.Request) {
	opts := getOptionsFromRequest(r)

	result, err := h.dataFactory.GetGitRepos(r.Context(), opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *APIHandler) getRunners(w http.ResponseWriter, r *http.Request) {
	opts := getOptionsFromRequest(r)

	result, err := h.dataFactory.GetRunners(r.Context(), opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// getDiscoveries returns a list of Discovery resources with optional filtering.
func (h *APIHandler) getDiscoveries(w http.ResponseWriter, r *http.Request) {
	opts := getOptionsFromRequest(r)

	result, err := h.dataFactory.GetDiscoveries(r.Context(), opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
