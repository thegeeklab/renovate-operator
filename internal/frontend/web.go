package frontend

import (
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ui_template "github.com/thegeeklab/renovate-operator/internal/frontend/template"
)

type WebView struct {
	Name          string
	Namespace     string
	GitRepoCount  int
	RunnerName    string
	DiscoveryName string
}

type WebHandler struct {
	client      client.Client
	dataFactory *DataFactory
	templates   *template.Template
}

func NewWebHandler(client client.Client) *WebHandler {
	tmpl, err := ui_template.Parse()
	if err != nil {
		panic(err)
	}

	return &WebHandler{
		client:      client,
		dataFactory: NewDataFactory(client),
		templates:   tmpl,
	}
}

func (h *WebHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/", h.HandleIndex).Methods("GET")

	partialsRouter := router.PathPrefix("/partials").Subrouter()
	partialsRouter.Use(h.EnsureHTMXRequest)
	partialsRouter.HandleFunc("/renovators", h.HandleRenovatorsPartial).Methods("GET")
	partialsRouter.HandleFunc("/gitrepos", h.HandleGitReposPartial).Methods("GET")
}

func (h *WebHandler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.templates.ExecuteTemplate(w, "index", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) HandleRenovatorsPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all renovators
	renovators, err := h.dataFactory.GetRenovators(ctx)
	if err != nil {
		http.Error(w, "Failed to list renovators", http.StatusInternalServerError)

		return
	}

	// Get all runners
	runners, err := h.dataFactory.GetRunners(ctx, "", "")
	if err != nil {
		http.Error(w, "Failed to list runners", http.StatusInternalServerError)

		return
	}

	// Get all discoveries
	discoveries, err := h.dataFactory.GetDiscoveries(ctx, "", "")
	if err != nil {
		http.Error(w, "Failed to list discoveries", http.StatusInternalServerError)

		return
	}

	// Get all git repos
	repos, err := h.dataFactory.GetGitRepos(ctx, "", "")
	if err != nil {
		http.Error(w, "Failed to list repos", http.StatusInternalServerError)

		return
	}

	runnersByNs := make(map[string]string)
	for _, runner := range runners {
		runnersByNs[runner.Namespace] = runner.Name
	}

	discoveriesByNs := make(map[string]string)
	for _, discovery := range discoveries {
		discoveriesByNs[discovery.Namespace] = discovery.Name
	}

	repoCountByNs := make(map[string]int)
	for _, repo := range repos {
		repoCountByNs[repo.Namespace]++
	}

	var views []WebView

	for _, ren := range renovators {
		view := WebView{
			Name:          ren.Name,
			Namespace:     ren.Namespace,
			GitRepoCount:  repoCountByNs[ren.Namespace],
			RunnerName:    runnersByNs[ren.Namespace],
			DiscoveryName: discoveriesByNs[ren.Namespace],
		}

		if view.RunnerName == "" {
			view.RunnerName = "-"
		}

		if view.DiscoveryName == "" {
			view.DiscoveryName = "-"
		}

		views = append(views, view)
	}

	w.Header().Set("Content-Type", "text/html")

	if err := h.templates.ExecuteTemplate(w, "renovator_list", views); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) HandleGitReposPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.URL.Query().Get("namespace")

	if namespace == "" {
		http.Error(w, "Namespace parameter is required", http.StatusBadRequest)

		return
	}

	// Get git repos for the specified namespace
	repos, err := h.dataFactory.GetGitRepos(ctx, namespace, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html")

	if err := h.templates.ExecuteTemplate(w, "gitrepo_list", repos); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) EnsureHTMXRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("HX-Request") != "true" {
			http.Redirect(w, r, "/", http.StatusFound)

			return
		}

		next.ServeHTTP(w, r)
	})
}
