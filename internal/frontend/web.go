package frontend

import (
	"html/template"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	ui_template "github.com/thegeeklab/renovate-operator/internal/frontend/template"
	"github.com/thegeeklab/renovate-operator/internal/logstore"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	logManager  *logstore.Manager
}

type GitRepoViewData struct {
	Repo GitRepoInfo
	Jobs []JobInfo
}

type JobLogData struct {
	JobName string
	Content string
	Error   string
}

func NewWebHandler(client client.Client, logManager *logstore.Manager) *WebHandler {
	tmpl, err := ui_template.Parse()
	if err != nil {
		panic(err)
	}

	return &WebHandler{
		client:      client,
		dataFactory: NewDataFactory(client),
		templates:   tmpl,
		logManager:  logManager,
	}
}

func (h *WebHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/", h.HandleIndex).Methods("GET")

	partialsRouter := router.PathPrefix("/partials").Subrouter()
	partialsRouter.Use(h.EnsureHTMXRequest)
	partialsRouter.HandleFunc("/renovators", h.HandleRenovatorsPartial).Methods("GET")
	partialsRouter.HandleFunc("/gitrepos", h.HandleGitReposPartial).Methods("GET")
	partialsRouter.HandleFunc("/gitrepo", h.HandleGitRepoView).Methods("GET")
	partialsRouter.HandleFunc("/joblogs", h.HandleJobLogs).Methods("GET")
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

// HandleGitRepoView renders a dedicated view for a GitRepo and its jobs.
func (h *WebHandler) HandleGitRepoView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")

	if namespace == "" || name == "" {
		http.Error(w, "Namespace and name parameters are required", http.StatusBadRequest)

		return
	}

	var repo renovatev1beta1.GitRepo
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &repo); err != nil {
		http.Error(w, "GitRepo not found", http.StatusNotFound)

		return
	}

	repoInfo := GitRepoInfo{
		Name:      repo.Name,
		Namespace: repo.Namespace,
		WebhookID: repo.Spec.WebhookID,
		Ready:     repo.Status.Ready,
		CreatedAt: repo.CreationTimestamp.Time,
	}

	jobs, err := h.dataFactory.GetJobsForRepo(ctx, namespace, name)
	if err != nil {
		http.Error(w, "Failed to fetch jobs", http.StatusInternalServerError)

		return
	}

	data := GitRepoViewData{
		Repo: repoInfo,
		Jobs: jobs,
	}

	w.Header().Set("Content-Type", "text/html")

	if err := h.templates.ExecuteTemplate(w, "gitrepo_view", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// HandleJobLogs fetches the log stream and renders it.
func (h *WebHandler) HandleJobLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.URL.Query().Get("namespace")
	runner := r.URL.Query().Get("runner")
	job := r.URL.Query().Get("job")

	if namespace == "" || runner == "" || job == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)

		return
	}

	data := JobLogData{
		JobName: job,
	}

	stream, err := h.logManager.GetLogStream(ctx, namespace, "runner", runner, job)
	if err != nil {
		data.Error = "Logs are no longer available. They may have been purged by " +
			"the background job or the Pod was deleted before archiving completed."
	} else {
		defer stream.Close()

		content, ioErr := io.ReadAll(stream)
		if ioErr != nil {
			data.Error = "Failed to read log stream from storage."
		} else {
			data.Content = string(content)
		}
	}

	w.Header().Set("Content-Type", "text/html")

	// Pass the struct to the template
	if err := h.templates.ExecuteTemplate(w, "job_logs", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
