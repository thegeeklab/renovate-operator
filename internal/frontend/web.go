package frontend

import (
	"io"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/frontend/views"
	"github.com/thegeeklab/renovate-operator/internal/logstore"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FrontendAssets struct {
	Styles  []string
	Scripts []string
}

type WebHandler struct {
	client      client.Client
	dataFactory *DataFactory
	logManager  *logstore.Manager
	Broker      *SSEBroker
	assets      FrontendAssets
}

func NewWebHandler(
	client client.Client, logManager *logstore.Manager, broker *SSEBroker, assets FrontendAssets,
) *WebHandler {
	return &WebHandler{
		client:      client,
		dataFactory: NewDataFactory(client),
		logManager:  logManager,
		Broker:      broker,
		assets:      assets,
	}
}

func (h *WebHandler) RegisterRoutes(router *mux.Router) {
	router.Handle("/events", h.Broker).Methods("GET")

	router.HandleFunc("/", h.HandleDashboard).Methods("GET")
	router.HandleFunc("/gitrepo", h.HandleGitRepoView).Methods("GET")
	router.HandleFunc("/gitrepos", h.HandleGitReposPartial).Methods("GET")
	router.HandleFunc("/joblogs", h.HandleJobLogs).Methods("GET")
}

func (h *WebHandler) render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	isHxRequest := r.Header.Get("HX-Request") == "true"
	isHxBoosted := r.Header.Get("HX-Boosted") == "true"

	if isHxRequest && !isHxBoosted {
		_ = component.Render(r.Context(), w)
	} else {
		_ = views.Layout(h.assets.Styles, h.assets.Scripts, component).Render(r.Context(), w)
	}
}

func getWebListOptions(r *http.Request) ListOptions {
	q := r.URL.Query()

	return ListOptions{
		Namespace: q.Get("namespace"),
		Renovator: q.Get("renovator"),
		SortBy:    q.Get("sort"),
		Order:     q.Get("order"),
	}
}

func (h *WebHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	opts := getWebListOptions(r)

	renovators, err := h.dataFactory.GetRenovators(ctx, opts)
	if err != nil {
		http.Error(w, "Failed to list renovators", http.StatusInternalServerError)

		return
	}

	var viewsList []views.WebView

	for _, ren := range renovators {
		renOpts := opts
		renOpts.Namespace = ren.Namespace
		renOpts.Renovator = ren.UID

		runners, _ := h.dataFactory.GetRunners(ctx, renOpts)
		discoveries, _ := h.dataFactory.GetDiscoveries(ctx, renOpts)
		repos, _ := h.dataFactory.GetGitRepos(ctx, renOpts)

		runnerName := "-"
		if len(runners) > 0 {
			runnerName = runners[0].Name
		}

		discoveryName := "-"
		if len(discoveries) > 0 {
			discoveryName = discoveries[0].Name
		}

		viewsList = append(viewsList, views.WebView{
			Name:          ren.Name,
			Namespace:     ren.Namespace,
			Renovator:     ren.UID,
			GitRepoCount:  len(repos),
			RunnerName:    runnerName,
			DiscoveryName: discoveryName,
		})
	}

	w.Header().Set("Content-Type", "text/html")
	h.render(w, r, views.RenovatorList(viewsList))
}

func (h *WebHandler) HandleGitReposPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	opts := getWebListOptions(r)

	if opts.Namespace == "" {
		http.Error(w, "Namespace parameter is required", http.StatusBadRequest)

		return
	}

	repos, err := h.dataFactory.GetGitRepos(ctx, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	var viewRepos []views.GitRepoInfo
	for _, repo := range repos {
		viewRepos = append(viewRepos, views.GitRepoInfo{
			Name:      repo.Name,
			Namespace: repo.Namespace,
			WebhookID: repo.WebhookID,
			Ready:     repo.Ready,
			CreatedAt: repo.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "text/html")
	_ = views.GitRepoList(viewRepos).Render(r.Context(), w)
}

func (h *WebHandler) HandleGitRepoView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	opts := getWebListOptions(r)
	name := r.URL.Query().Get("name")

	if opts.Namespace == "" || name == "" {
		http.Error(w, "Namespace and name parameters are required", http.StatusBadRequest)

		return
	}

	var repo renovatev1beta1.GitRepo
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: opts.Namespace, Name: name}, &repo); err != nil {
		http.Error(w, "GitRepo not found", http.StatusNotFound)

		return
	}

	repoInfo := views.GitRepoInfo{
		Name:      repo.Name,
		Namespace: repo.Namespace,
		WebhookID: repo.Spec.WebhookID,
		Ready:     repo.Status.Ready,
		CreatedAt: repo.CreationTimestamp.Time,
	}

	jobs, err := h.dataFactory.GetJobsForRepo(ctx, name, opts)
	if err != nil {
		http.Error(w, "Failed to fetch jobs", http.StatusInternalServerError)

		return
	}

	var viewJobs []views.JobInfo
	for _, j := range jobs {
		viewJobs = append(viewJobs, views.JobInfo{
			Name:      j.Name,
			Namespace: j.Namespace,
			Runner:    j.Runner,
			Status:    j.Status,
			CreatedAt: j.CreatedAt,
		})
	}

	data := views.GitRepoViewData{
		Repo: repoInfo,
		Jobs: viewJobs,
	}

	w.Header().Set("Content-Type", "text/html")
	h.render(w, r, views.GitRepoView(data))
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

	data := views.JobLogData{
		JobName:   job,
		Namespace: namespace,
		Runner:    runner,
	}

	var k8sJob batchv1.Job
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: job}, &k8sJob); err == nil {
		if k8sJob.Status.CompletionTime == nil && k8sJob.Status.Failed == 0 {
			data.IsRunning = true
		}
	}

	stream, err := h.logManager.GetLogStream(ctx, namespace, "runner", runner, job)
	if err != nil {
		data.Error = "Logs are no longer available. They may have been purged by " +
			"the background job or the Pod was deleted before archiving completed."

		if data.IsRunning {
			data.Content = "Waiting for logs to initialize..."
		}
	} else {
		defer stream.Close()

		content, ioErr := io.ReadAll(stream)
		if ioErr != nil {
			data.Error = "Failed to read log stream from storage."
		} else {
			data.Content = string(content)
		}
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0")
	w.Header().Set("Content-Type", "text/html")
	_ = views.JobLogs(data).Render(r.Context(), w)
}
