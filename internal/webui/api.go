package webui

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/thegeeklab/renovate-operator/api/v1beta1"
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
	Strategy  string    `json:"strategy"`
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
	client client.Client
}

// NewAPIHandler creates a new APIHandler.
func NewAPIHandler(client client.Client) *APIHandler {
	return &APIHandler{
		client: client,
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

	var renovatorList v1beta1.RenovatorList
	if err := h.client.List(ctx, &renovatorList); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	var result []RenovatorInfo
	for _, renovator := range renovatorList.Items {
		result = append(result, RenovatorInfo{
			Name:      renovator.Name,
			Namespace: renovator.Namespace,
			Schedule:  renovator.Spec.Schedule,
			Ready:     renovator.Status.Ready,
			CreatedAt: renovator.CreationTimestamp.Time,
		})
	}

	w.Header().Set("Content-Type", "application/json")

	// Ensure empty slice is returned as [] instead of null
	if result == nil {
		result = []RenovatorInfo{}
	}

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// getGitRepos returns a list of GitRepo resources with optional filtering.
func (h *APIHandler) getGitRepos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters for filtering
	queryParams := r.URL.Query()
	namespace := queryParams.Get("namespace")
	renovator := queryParams.Get("renovator")

	var gitRepoList v1beta1.GitRepoList
	if err := h.client.List(ctx, &gitRepoList); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	var result []GitRepoInfo

	for _, gitrepo := range gitRepoList.Items {
		// Apply filters if provided
		if (namespace != "" && gitrepo.Namespace != namespace) ||
			(renovator != "" && gitrepo.Namespace != renovator) {
			continue
		}

		result = append(result, GitRepoInfo{
			Name:      gitrepo.Name,
			Namespace: gitrepo.Namespace,
			WebhookID: gitrepo.Spec.WebhookID,
			Ready:     gitrepo.Status.Ready,
			CreatedAt: gitrepo.CreationTimestamp.Time,
		})
	}

	w.Header().Set("Content-Type", "application/json")

	// Ensure empty slice is returned as [] instead of null
	if result == nil {
		result = []GitRepoInfo{}
	}

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *APIHandler) getRunners(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters for filtering
	queryParams := r.URL.Query()
	namespace := queryParams.Get("namespace")
	renovator := queryParams.Get("renovator")

	var runnerList v1beta1.RunnerList
	if err := h.client.List(ctx, &runnerList); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	var result []RunnerInfo

	for _, runner := range runnerList.Items {
		// Apply filters if provided
		if (namespace != "" && runner.Namespace != namespace) ||
			(renovator != "" && runner.Namespace != renovator) {
			continue
		}

		var strategy string
		if runner.Spec.Strategy != "" {
			strategy = string(runner.Spec.Strategy)
		}

		result = append(result, RunnerInfo{
			Name:      runner.Name,
			Namespace: runner.Namespace,
			Strategy:  strategy,
			Instances: runner.Spec.Instances,
			Ready:     runner.Status.Ready,
			CreatedAt: runner.CreationTimestamp.Time,
		})
	}

	w.Header().Set("Content-Type", "application/json")

	// Ensure empty slice is returned as [] instead of null
	if result == nil {
		result = []RunnerInfo{}
	}

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// getDiscoveries returns a list of Discovery resources with optional filtering.
func (h *APIHandler) getDiscoveries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters for filtering
	queryParams := r.URL.Query()
	namespace := queryParams.Get("namespace")
	renovator := queryParams.Get("renovator")

	var discoveryList v1beta1.DiscoveryList
	if err := h.client.List(ctx, &discoveryList); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	var result []DiscoveryInfo

	for _, discovery := range discoveryList.Items {
		// Apply filters if provided
		if (namespace != "" && discovery.Namespace != namespace) ||
			(renovator != "" && discovery.Namespace != renovator) {
			continue
		}

		result = append(result, DiscoveryInfo{
			Name:      discovery.Name,
			Namespace: discovery.Namespace,
			Schedule:  discovery.Spec.Schedule,
			Ready:     discovery.Status.Ready,
			CreatedAt: discovery.CreationTimestamp.Time,
		})
	}

	w.Header().Set("Content-Type", "application/json")

	// Ensure empty slice is returned as [] instead of null
	if result == nil {
		result = []DiscoveryInfo{}
	}

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
