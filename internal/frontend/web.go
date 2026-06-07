package frontend

import (
	"context"
	"io"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
	batchv1 "k8s.io/api/batch/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/auth"
	"github.com/thegeeklab/renovate-operator/internal/frontend/view"
	"github.com/thegeeklab/renovate-operator/internal/frontend/viewmodel"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
)

type FrontendAssets struct {
	Styles  []string
	Scripts []string
}

type WebHandler struct {
	client      client.Client
	dataFactory *DataFactory
	Broker      *SSEBroker
	assets      FrontendAssets
	authManager *auth.Manager
}

func NewWebHandler(
	client client.Client,
	clientset kubernetes.Interface,
	broker *SSEBroker,
	assets FrontendAssets,
	authManager *auth.Manager,
) *WebHandler {
	return &WebHandler{
		client:      client,
		dataFactory: NewDataFactory(client, clientset, authManager),
		Broker:      broker,
		assets:      assets,
		authManager: authManager,
	}
}

const (
	maxLogReadSize                  = 2 * 1024 * 1024 // 2MB
	maxConcurrentRenovatorSummaries = 10
)

func (h *WebHandler) RegisterRoutes(router *mux.Router) {
	router.Handle("/events", h.Broker).Methods("GET")

	router.HandleFunc("/", h.HandleDashboard).Methods("GET")
	router.HandleFunc("/login", h.HandleLogin).Methods("GET")
	router.HandleFunc("/gitrepo", h.HandleGitRepoView).Methods("GET")
	router.HandleFunc("/gitrepos", h.HandleGitReposPartial).Methods("GET")
	router.HandleFunc("/joblogs", h.HandleJobLogs).Methods("GET")
	router.HandleFunc("/joblogs/download", h.HandleJobLogsDownload).Methods("GET")
}

func (h *WebHandler) render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	isHxRequest := r.Header.Get("HX-Request") == "true"
	isHxBoosted := r.Header.Get("HX-Boosted") == "true"

	authInfo := h.buildAuthInfo(r)

	if isHxRequest && !isHxBoosted {
		_ = component.Render(r.Context(), w)
	} else {
		_ = view.Layout(h.assets.Styles, h.assets.Scripts, authInfo, component).Render(r.Context(), w)
	}
}

func (h *WebHandler) buildAuthInfo(r *http.Request) viewmodel.AuthInfo {
	info := viewmodel.AuthInfo{}

	if h.authManager == nil || !h.authManager.IsEnabled() {
		return info
	}

	info.Enabled = true

	for _, p := range h.authManager.List() {
		info.Providers = append(info.Providers, viewmodel.AuthProviderInfo{
			Name: p.Name(),
		})
	}

	session, ok := auth.GetSessionData(r.Context(), h.authManager.Session)
	if !ok {
		return info
	}

	info.Authenticated = true
	info.Name = session.Name
	info.Provider = session.Provider

	csrfToken := auth.GetCSRFToken(r.Context(), h.authManager.Session)
	if csrfToken != "" {
		info.CSRFToken = csrfToken
	}

	return info
}

// requireAuth aborts the response with a redirect to /login if auth is enabled
// and the request is not authenticated. Returns true if the handler may proceed.
func (h *WebHandler) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	authInfo := h.buildAuthInfo(r)
	if h.authManager != nil && h.authManager.IsEnabled() && !authInfo.Authenticated {
		http.Redirect(w, r, "/login", http.StatusFound)

		return false
	}

	return true
}

// toViewGitRepoInfo converts an API-layer GitRepoInfo into a view-layer one.
func toViewGitRepoInfo(r GitRepoInfo) viewmodel.GitRepoInfo {
	return viewmodel.GitRepoInfo{
		Name:               r.Name,
		FullName:           r.FullName,
		Namespace:          r.Namespace,
		RenovatorName:      r.RenovatorName,
		WebhookID:          r.WebhookID,
		LastRenovateAt:     r.LastRenovateAt,
		LastRenovateStatus: r.LastRenovateStatus,
		CreatedAt:          r.CreatedAt,
	}
}

func toViewJobInfo(j JobInfo) viewmodel.JobInfo {
	return viewmodel.JobInfo{
		Name:      j.Name,
		Namespace: j.Namespace,
		Runner:    j.Runner,
		Status:    j.Status,
		CreatedAt: j.CreatedAt,
	}
}

func (h *WebHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	opts := getOptionsFromRequest(r)
	searchQuery := opts.Search

	if searchQuery != "" {
		repos, err := h.dataFactory.GetGitRepos(ctx, opts)
		if err != nil {
			frontendLog.Error(err, "Failed to search repositories")
			http.Error(w, "Failed to search repositories", http.StatusInternalServerError)

			return
		}

		repos = h.dataFactory.ApplyAccessFilter(ctx, repos)

		viewRepos := make([]viewmodel.GitRepoInfo, 0, len(repos))
		for _, repo := range repos {
			viewRepos = append(viewRepos, toViewGitRepoInfo(repo))
		}

		w.Header().Set("Content-Type", "text/html")
		h.render(w, r, view.RenovatorList(nil, searchQuery, viewRepos))

		return
	}

	renovators, err := h.dataFactory.GetRenovators(ctx, opts)
	if err != nil {
		frontendLog.Error(err, "Failed to list renovators")
		http.Error(w, "Failed to list renovators", http.StatusInternalServerError)

		return
	}

	summaries := h.buildRenovatorSummaries(ctx, renovators, opts)

	w.Header().Set("Content-Type", "text/html")
	h.render(w, r, view.RenovatorList(summaries, searchQuery, nil))
}

// buildRenovatorSummaries fetches the runner, discovery, and repo count for each
// Renovator in parallel. Per-renovator fetches are independent of one another, and
// within a single renovator the three queries are independent, so both axes are
// fanned out. A failure in one query degrades to "-" placeholders for that
// renovator rather than aborting the whole dashboard.
func (h *WebHandler) buildRenovatorSummaries(
	ctx context.Context,
	renovators []RenovatorInfo,
	opts ListOptions,
) []viewmodel.WebView {
	summaries := make([]viewmodel.WebView, len(renovators))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(maxConcurrentRenovatorSummaries)

	for i, ren := range renovators {
		group.Go(func() error {
			renOpts := opts
			renOpts.Namespace = ren.Namespace
			renOpts.Renovator = ren.UID

			var (
				runners     []RunnerInfo
				discoveries []DiscoveryInfo
				repos       []GitRepoInfo
			)

			innerGroup, innerCtx := errgroup.WithContext(groupCtx)
			innerGroup.Go(func() error {
				var err error

				runners, err = h.dataFactory.GetRunners(innerCtx, renOpts)

				return err
			})
			innerGroup.Go(func() error {
				var err error

				discoveries, err = h.dataFactory.GetDiscoveries(innerCtx, renOpts)

				return err
			})
			innerGroup.Go(func() error {
				var err error

				repos, err = h.dataFactory.GetGitRepos(innerCtx, renOpts)

				return err
			})

			if err := innerGroup.Wait(); err != nil {
				frontendLog.Error(err, "Failed to load renovator summary data",
					"renovator", ren.Name, "namespace", ren.Namespace)
			}

			if repos != nil {
				repos = h.dataFactory.ApplyAccessFilter(ctx, repos)
			}

			summary := viewmodel.WebView{
				Name:          ren.Name,
				Namespace:     ren.Namespace,
				Renovator:     ren.UID,
				GitRepoCount:  len(repos),
				RunnerName:    "-",
				DiscoveryName: "-",
			}
			if len(runners) > 0 {
				summary.RunnerName = runners[0].Name
			}

			if len(discoveries) > 0 {
				summary.DiscoveryName = discoveries[0].Name
			}

			summaries[i] = summary

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		frontendLog.Error(err, "Unexpected error building renovator summaries")
	}

	return summaries
}

func (h *WebHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, view.Login(h.buildAuthInfo(r)))
}

func (h *WebHandler) HandleGitReposPartial(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	opts := getOptionsFromRequest(r)

	if opts.Namespace == "" {
		http.Error(w, "Namespace parameter is required", http.StatusBadRequest)

		return
	}

	repos, err := h.dataFactory.GetGitRepos(ctx, opts)
	if err != nil {
		frontendLog.Error(err, "Failed to list git repos", "namespace", opts.Namespace)
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	repos = h.dataFactory.ApplyAccessFilter(ctx, repos)

	viewRepos := make([]viewmodel.GitRepoInfo, 0, len(repos))
	for _, repo := range repos {
		viewRepos = append(viewRepos, toViewGitRepoInfo(repo))
	}

	w.Header().Set("Content-Type", "text/html")
	_ = view.GitRepoList(viewRepos).Render(r.Context(), w)
}

func (h *WebHandler) HandleGitRepoView(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	opts := getOptionsFromRequest(r)
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

	repoInfo := viewmodel.GitRepoInfo{
		Name:      repo.Name,
		FullName:  repo.Spec.Name,
		Namespace: repo.Namespace,
		WebhookID: repo.Status.WebhookID,
		CreatedAt: repo.CreationTimestamp.Time,
	}

	repoInfo.LastRenovateStatus, repoInfo.LastRenovateAt = getRenovateStatusFromConditions(&repo)

	if !h.dataFactory.IsUserRepo(ctx, repoInfo.FullName) {
		http.Error(w, "GitRepo not found", http.StatusNotFound)

		return
	}

	jobs, err := h.dataFactory.GetJobsForRepo(ctx, name, opts)
	if err != nil {
		frontendLog.Error(err, "Failed to fetch jobs", "repo", name, "namespace", opts.Namespace)
		http.Error(w, "Failed to fetch jobs", http.StatusInternalServerError)

		return
	}

	viewJobs := make([]viewmodel.JobInfo, 0, len(jobs))
	for _, j := range jobs {
		viewJobs = append(viewJobs, toViewJobInfo(j))
	}

	data := viewmodel.GitRepoViewData{
		Repo: repoInfo,
		Jobs: viewJobs,
	}

	w.Header().Set("Content-Type", "text/html")
	h.render(w, r, view.GitRepoView(data))
}

func (h *WebHandler) buildJobLogData(ctx context.Context, namespace, runner, job string) viewmodel.JobLogData {
	data := viewmodel.JobLogData{
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

	stream, err := h.dataFactory.GetJobLogs(ctx, namespace, job)
	if err != nil {
		if data.IsRunning && api_errors.IsNotFound(err) {
			data.Error = ""
			data.Content = "Waiting for pods to initialize..."
		} else {
			data.Error = "Failed to fetch logs: " + err.Error()
		}

		return data
	}
	defer stream.Close()

	content, ioErr := io.ReadAll(io.LimitReader(stream, maxLogReadSize))
	if ioErr != nil {
		data.Error = "Failed to read log stream from pod."

		return data
	}

	data.Content = string(content)
	if len(content) == maxLogReadSize {
		data.Content += "\n--- Log truncated at 2MB ---"
	}

	return data
}

// HandleJobLogs fetches the log stream and renders it.
func (h *WebHandler) HandleJobLogs(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	namespace := r.URL.Query().Get("namespace")
	runner := r.URL.Query().Get("runner")
	job := r.URL.Query().Get("job")

	if namespace == "" || runner == "" || job == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)

		return
	}

	data := h.buildJobLogData(ctx, namespace, runner, job)

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "text/html")
	_ = view.JobLogs(data).Render(r.Context(), w)
}

func (h *WebHandler) HandleJobLogsDownload(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuth(w, r) {
		return
	}

	ctx := r.Context()
	namespace := r.URL.Query().Get("namespace")
	job := r.URL.Query().Get("job")

	if namespace == "" || job == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)

		return
	}

	stream, err := h.dataFactory.GetJobLogs(ctx, namespace, job)
	if err != nil {
		var k8sJob batchv1.Job

		isRunning := false

		if err := h.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: job}, &k8sJob); err == nil {
			if k8sJob.Status.CompletionTime == nil && k8sJob.Status.Failed == 0 {
				isRunning = true
			}
		}

		if isRunning && api_errors.IsNotFound(err) {
			http.Error(w, "Logs are not yet available. The pods may still be initializing.", http.StatusNotFound)

			return
		}

		http.Error(w, "Logs are no longer available. The pods may have been garbage collected.", http.StatusNotFound)

		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	safeJobName := "job"
	if sanitized, err := k8s.SanitizeName(job); err == nil && sanitized != "" {
		safeJobName = sanitized
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+safeJobName+".log\"")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	if _, err := io.Copy(w, stream); err != nil {
		frontendLog.Error(err, "Failed to stream job logs download", "job", job)
	}
}
