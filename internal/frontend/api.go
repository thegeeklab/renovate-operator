package frontend

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// API response types for the web UI

type RenovatorInfo struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
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
	Instances int32     `json:"instances"`
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

// APIHandler manages the web UI API endpoints.
type APIHandler struct {
	client      client.Client
	dataFactory *DataFactory
}

// NewAPIHandler creates a new APIHandler.
func NewAPIHandler(client client.Client) *APIHandler {
	return &APIHandler{
		client:      client,
		dataFactory: NewDataFactory(client),
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

// getVersion returns the API version information.
func (h *APIHandler) getVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(struct {
		Version string `json:"version"`
	}{
		Version: "v1.0.0",
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}
}

// getRenovators returns a list of all Renovator resources.
func (h *APIHandler) getRenovators(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	result, err := h.dataFactory.GetRenovators(ctx)
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
	ctx := r.Context()

	queryParams := r.URL.Query()
	namespace := queryParams.Get("namespace")
	renovator := queryParams.Get("renovator")

	result, err := h.dataFactory.GetGitRepos(ctx, namespace, renovator)
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
	ctx := r.Context()

	queryParams := r.URL.Query()
	namespace := queryParams.Get("namespace")
	renovator := queryParams.Get("renovator")

	result, err := h.dataFactory.GetRunners(ctx, namespace, renovator)
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
	ctx := r.Context()

	queryParams := r.URL.Query()
	namespace := queryParams.Get("namespace")
	renovator := queryParams.Get("renovator")

	result, err := h.dataFactory.GetDiscoveries(ctx, namespace, renovator)
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

		return
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

		return
	}
}
