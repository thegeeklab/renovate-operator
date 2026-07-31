package frontend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/maypok86/otter/v2"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/frontend/auth"
	"github.com/thegeeklab/renovate-operator/internal/frontend/viewmodel"
	"github.com/thegeeklab/renovate-operator/internal/logreader"
	"github.com/thegeeklab/renovate-operator/internal/parser"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errLogReaderNotConfigured = errors.New("log reader not configured")
	errUnableToDeriveCacheKey = errors.New("unable to derive cache key for session")
	errUnexpectedCacheResult  = errors.New("unexpected cache result type")
	errAuthNotEnabled         = errors.New("auth not enabled")
	errAuthNotReady           = errors.New("auth not ready")
	errNotAuthenticated       = errors.New("not authenticated")
)

// ListOptions holds optional parameters for filtering and sorting data.
type ListOptions struct {
	Namespace string
	Renovator string
	SortBy    string
	Order     string
	Search    string
}

const (
	defaultAccessCacheTTL               = 60 * time.Second
	defaultAccessCacheMax               = 500
	defaultAuthorizedRenovatorsCacheTTL = 30 * time.Second
	defaultAuthorizedRenovatorsCacheMax = 500
	defaultHTTPClientCacheTTL           = 24 * time.Hour
	defaultHTTPClientCacheMax           = 1000
	defaultHTTPClientTimeout            = 30 * time.Second
	defaultPRActivityCacheTTL           = 30 * time.Second
	defaultPRActivityCacheMax           = 500
	defaultPRActivityConcurrency        = 4
)

func (df *DataFactory) deriveCacheKey(session auth.SessionData) string {
	if session.Subject != "" {
		return session.Provider + "|" + session.Subject
	}

	if session.AccessToken != "" {
		return session.Provider + "|token:" + auth.HashAccessToken(session.AccessToken)
	}

	return ""
}

// DataFactory provides methods to fetch and transform data for both API and UI handlers.
type DataFactory struct {
	client                    client.Client
	clientset                 kubernetes.Interface
	logReader                 logreader.Reader
	authManager               *auth.Manager
	accessCache               *otter.Cache[string, map[string]bool]
	accessGroup               singleflight.Group
	authorizedRenovatorsCache *otter.Cache[string, []string]
	authorizedRenovatorsGroup singleflight.Group
	httpClientCache           *otter.Cache[string, *http.Client]
	prActivityCache           *otter.Cache[string, map[string]PerRepoActivity]
	prActivityGroup           singleflight.Group
}

// NewDataFactory creates a new DataFactory instance.
func NewDataFactory(
	client client.Client,
	clientset kubernetes.Interface,
	authManager *auth.Manager,
	logReader logreader.Reader,
) *DataFactory {
	accessCache := otter.Must(&otter.Options[string, map[string]bool]{
		ExpiryCalculator: otter.ExpiryAccessing[string, map[string]bool](defaultAccessCacheTTL),
		MaximumSize:      defaultAccessCacheMax,
	})

	authorizedRenovatorsCache := otter.Must(&otter.Options[string, []string]{
		ExpiryCalculator: otter.ExpiryWriting[string, []string](defaultAuthorizedRenovatorsCacheTTL),
		MaximumSize:      defaultAuthorizedRenovatorsCacheMax,
	})

	httpClientCache := otter.Must(&otter.Options[string, *http.Client]{
		ExpiryCalculator: otter.ExpiryAccessing[string, *http.Client](defaultHTTPClientCacheTTL),
		MaximumSize:      defaultHTTPClientCacheMax,
	})

	prActivityCache := otter.Must(&otter.Options[string, map[string]PerRepoActivity]{
		ExpiryCalculator: otter.ExpiryWriting[string, map[string]PerRepoActivity](defaultPRActivityCacheTTL),
		MaximumSize:      defaultPRActivityCacheMax,
	})

	return &DataFactory{
		client:                    client,
		clientset:                 clientset,
		logReader:                 logReader,
		authManager:               authManager,
		accessCache:               accessCache,
		authorizedRenovatorsCache: authorizedRenovatorsCache,
		httpClientCache:           httpClientCache,
		prActivityCache:           prActivityCache,
	}
}

// prActivityCacheKey returns the cache key for a (namespace, renovatorUID)
// pair, scoped per user so per-repo access filtering cannot leak between sessions.
func (df *DataFactory) prActivityCacheKey(ctx context.Context, namespace, renovatorUID string) string {
	user := "anon"

	if df.authManager != nil && df.authManager.IsEnabled() {
		session, ok := auth.GetSessionData(ctx, df.authManager.SessionManager())
		if ok {
			if key := df.deriveCacheKey(session); key != "" {
				user = key
			}
		}
	}

	return namespace + "|" + renovatorUID + "|" + user
}

// buildListOptions creates standard client.ListOptions for server-side filtering.
func buildListOptions(opt ListOptions) []client.ListOption {
	var listOpts []client.ListOption

	if opt.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(opt.Namespace))
	}

	if opt.Renovator != "" {
		listOpts = append(listOpts, client.MatchingLabels{
			renovatev1beta1.LabelRenovator: opt.Renovator,
		})
	}

	return listOpts
}

// getAuthorizedRenovatorUIDs returns the UIDs of Renovators the user has access to.
// Returns nil if auth is disabled (meaning all resources are accessible).
// Returns error if auth is intended but not yet ready (fail closed).
// Results are cached to avoid redundant API calls when fetching multiple resource types.
func (df *DataFactory) getAuthorizedRenovatorUIDs(ctx context.Context) ([]string, error) {
	if df.authManager == nil {
		return nil, nil
	}

	if err := df.checkAuthReady(); err != nil {
		return nil, err
	}

	if !df.authManager.IsEnabled() {
		return nil, nil
	}

	session, ok := auth.GetSessionData(ctx, df.authManager.SessionManager())
	if !ok {
		return nil, errNotAuthenticated
	}

	cacheKey := df.deriveCacheKey(session)
	if cacheKey == "" {
		return nil, errUnableToDeriveCacheKey
	}

	// Detach cancellation from the shared loader: singleflight shares one
	// in-flight fetch across all callers with the same key. Using the first
	// caller's context directly would let that caller's cancellation fail the
	// fetch for every other concurrent caller. Values (including session data)
	// are preserved so GetRenovators can still authenticate.
	loaderCtx := context.WithoutCancel(ctx)

	// Use singleflight to deduplicate concurrent requests for the same cache key
	result, err, _ := df.authorizedRenovatorsGroup.Do(cacheKey, func() (any, error) {
		loader := otter.LoaderFunc[string, []string](func(_ context.Context, _ string) ([]string, error) {
			// Cache miss - fetch from API
			renovators, err := df.GetRenovators(loaderCtx, ListOptions{})
			if err != nil {
				return nil, err
			}

			uidList := make([]string, 0, len(renovators))
			for _, r := range renovators {
				uidList = append(uidList, r.UID)
			}

			return uidList, nil
		})

		return df.authorizedRenovatorsCache.Get(loaderCtx, cacheKey, loader)
	})
	if err != nil {
		return nil, err
	}

	uidList, ok := result.([]string)
	if !ok {
		return nil, errUnexpectedCacheResult
	}

	return uidList, nil
}

// authorizeAndFilter applies authorization filtering to a list of resources.
// It checks if the user has access to the requested Renovator (if specified) and
// filters the results to only include resources from authorized Renovators.
// If authorizedUIDs is nil (auth disabled), all resources are returned.
func authorizeAndFilter[T any](
	opt ListOptions,
	items []T,
	getUID func(T) string,
	authorizedUIDs []string,
) []T {
	// If a specific Renovator is requested, verify the user has access to it
	if opt.Renovator != "" && authorizedUIDs != nil {
		if !slices.Contains(authorizedUIDs, opt.Renovator) {
			// User doesn't have access to this Renovator, return empty results
			return []T{}
		}
	}

	// Filter by authorized Renovators
	if authorizedUIDs == nil {
		return items
	}

	// Build the authorized UIDs set once
	uidSet := make(map[string]bool, len(authorizedUIDs))
	for _, uid := range authorizedUIDs {
		uidSet[uid] = true
	}

	filtered := make([]T, 0, len(items))
	for _, item := range items {
		if uidSet[getUID(item)] {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// GetRenovators fetches Renovator resources and transforms them into RenovatorInfo.
// When auth is enabled, it filters Renovators by the user's AuthProvider.
func (df *DataFactory) GetRenovators(ctx context.Context, opts ...ListOptions) ([]RenovatorInfo, error) {
	opt := getListOptions(opts)

	var listOpts []client.ListOption
	if opt.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(opt.Namespace))
	}

	// If auth is enabled, filter by the user's AuthProvider
	if df.authManager != nil && df.authManager.IsEnabled() {
		session, ok := auth.GetSessionData(ctx, df.authManager.SessionManager())
		if !ok {
			return nil, errNotAuthenticated
		}

		// Filter Renovators by AuthProvider label
		providerLabel, err := k8s.SanitizeLabel(session.Provider)
		if err != nil {
			return nil, fmt.Errorf("failed to sanitize provider label: %w", err)
		}

		listOpts = append(listOpts, client.MatchingLabels{
			renovatev1beta1.LabelAuthProvider: providerLabel,
		})
	} else if err := df.checkAuthReady(); err != nil {
		return nil, err
	}

	var list renovatev1beta1.RenovatorList
	if err := df.client.List(ctx, &list, listOpts...); err != nil {
		return nil, err
	}

	var result []RenovatorInfo
	for _, renovator := range list.Items {
		result = append(result, RenovatorInfo{
			Name:      renovator.Name,
			Namespace: renovator.Namespace,
			UID:       string(renovator.UID),
			Schedule:  renovator.Spec.Schedule,
			CreatedAt: renovator.CreationTimestamp.Time,
		})
	}

	result = util.EmptyIfNil(result)

	util.SortItems(
		result,
		util.SortBy(opt.SortBy),
		util.SortOrder(opt.Order),
		func(i RenovatorInfo) string { return i.Name },
		func(i RenovatorInfo) time.Time { return i.CreatedAt },
	)

	return result, nil
}

// GetGitRepo fetches a single GitRepo resource by namespace and name.
func (df *DataFactory) GetGitRepo(ctx context.Context, namespace, name string) (*viewmodel.GitRepoInfo, error) {
	var gitrepo renovatev1beta1.GitRepo
	if err := df.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &gitrepo); err != nil {
		return nil, err
	}

	info := gitRepoToInfo(&gitrepo)

	return &info, nil
}

// GetGitRepos fetches GitRepo resources with optional filtering.
func (df *DataFactory) GetGitRepos(ctx context.Context, opts ...ListOptions) ([]viewmodel.GitRepoInfo, error) {
	opt := getListOptions(opts)
	listOpts := buildListOptions(opt)

	var list renovatev1beta1.GitRepoList
	if err := df.client.List(ctx, &list, listOpts...); err != nil {
		return nil, err
	}

	var result []viewmodel.GitRepoInfo

	for _, gitrepo := range list.Items {
		// Skip GitRepos that are being deleted
		if !gitrepo.DeletionTimestamp.IsZero() {
			continue
		}

		result = append(result, gitRepoToInfo(&gitrepo))
	}

	// Apply authorization filtering when auth is enabled
	authorizedUIDs, err := df.getAuthorizedRenovatorUIDs(ctx)
	if err != nil {
		return nil, err
	}

	result = authorizeAndFilter(opt, result, func(r viewmodel.GitRepoInfo) string {
		return r.RenovatorUID
	}, authorizedUIDs)

	if opt.Search != "" {
		term := strings.ToLower(opt.Search)

		filtered := make([]viewmodel.GitRepoInfo, 0, len(result))
		for _, repo := range result {
			if strings.Contains(strings.ToLower(repo.Name), term) || strings.Contains(strings.ToLower(repo.FullName), term) {
				filtered = append(filtered, repo)
			}
		}

		result = filtered
	}

	result = util.EmptyIfNil(result)

	util.SortItems(
		result,
		util.SortBy(opt.SortBy),
		util.SortOrder(opt.Order),
		func(i viewmodel.GitRepoInfo) string { return i.Name },
		func(i viewmodel.GitRepoInfo) time.Time { return i.CreatedAt },
		func(i viewmodel.GitRepoInfo) time.Time { return i.LastRenovateAt },
	)

	return result, nil
}

// gitRepoToInfo converts a GitRepo CRD to a GitRepoInfo view model.
func gitRepoToInfo(gitrepo *renovatev1beta1.GitRepo) viewmodel.GitRepoInfo {
	lastStatus, lastTime := getRenovateStatusFromConditions(gitrepo)

	return viewmodel.GitRepoInfo{
		Name:               gitrepo.Name,
		FullName:           gitrepo.Spec.Name,
		Namespace:          gitrepo.Namespace,
		WebhookID:          gitrepo.Status.WebhookID,
		Platform:           gitrepo.Status.Platform,
		RepoURL:            gitrepo.Status.RepoURL,
		LastRenovateAt:     lastTime,
		LastRenovateStatus: lastStatus,
		CreatedAt:          gitrepo.CreationTimestamp.Time,
		RenovatorUID:       extractRenovatorUID(gitrepo.Labels),
	}
}

func getRenovateStatusFromConditions(repo *renovatev1beta1.GitRepo) (viewmodel.Status, time.Time) {
	var lastTime time.Time
	if repo.Status.LastRenovateTime != nil {
		lastTime = repo.Status.LastRenovateTime.Time
	}

	statusByType := map[string]viewmodel.Status{
		renovatev1beta1.GitRepoConditionRenovateRunning:   viewmodel.StatusRunning,
		renovatev1beta1.GitRepoConditionRenovateCompleted: viewmodel.StatusSucceeded,
		renovatev1beta1.GitRepoConditionRenovateFailed:    viewmodel.StatusFailed,
	}

	var (
		activeStatus     viewmodel.Status
		activeTransition time.Time
	)

	for condType, label := range statusByType {
		cond := repo.GetCondition(condType)
		if cond == nil || cond.Status != metav1.ConditionTrue {
			continue
		}

		if activeStatus == "" || cond.LastTransitionTime.After(activeTransition) {
			activeStatus = label
			activeTransition = cond.LastTransitionTime.Time
		}
	}

	if activeStatus == "" {
		return viewmodel.StatusUnknown, lastTime
	}

	return activeStatus, lastTime
}

// GetRunners fetches Runner resources with optional filtering.
func (df *DataFactory) GetRunners(ctx context.Context, opts ...ListOptions) ([]RunnerInfo, error) {
	opt := getListOptions(opts)
	listOpts := buildListOptions(opt)

	var list renovatev1beta1.RunnerList
	if err := df.client.List(ctx, &list, listOpts...); err != nil {
		return nil, err
	}

	var result []RunnerInfo

	for _, runner := range list.Items {
		result = append(result, RunnerInfo{
			Name:         runner.Name,
			Namespace:    runner.Namespace,
			CreatedAt:    runner.CreationTimestamp.Time,
			RenovatorUID: extractRenovatorUID(runner.Labels),
		})
	}

	// Apply authorization filtering when auth is enabled
	authorizedUIDs, err := df.getAuthorizedRenovatorUIDs(ctx)
	if err != nil {
		return nil, err
	}

	result = authorizeAndFilter(opt, result, func(r RunnerInfo) string {
		return r.RenovatorUID
	}, authorizedUIDs)

	result = util.EmptyIfNil(result)

	util.SortItems(
		result,
		util.SortBy(opt.SortBy),
		util.SortOrder(opt.Order),
		func(i RunnerInfo) string { return i.Name },
		func(i RunnerInfo) time.Time { return i.CreatedAt },
	)

	return result, nil
}

// GetDiscoveries fetches Discovery resources with optional filtering.
func (df *DataFactory) GetDiscoveries(ctx context.Context, opts ...ListOptions) ([]DiscoveryInfo, error) {
	opt := getListOptions(opts)
	listOpts := buildListOptions(opt)

	var list renovatev1beta1.DiscoveryList
	if err := df.client.List(ctx, &list, listOpts...); err != nil {
		return nil, err
	}

	var result []DiscoveryInfo

	for _, discovery := range list.Items {
		result = append(result, DiscoveryInfo{
			Name:         discovery.Name,
			Namespace:    discovery.Namespace,
			Schedule:     discovery.Spec.Schedule,
			CreatedAt:    discovery.CreationTimestamp.Time,
			RenovatorUID: extractRenovatorUID(discovery.Labels),
		})
	}

	// Apply authorization filtering when auth is enabled
	authorizedUIDs, err := df.getAuthorizedRenovatorUIDs(ctx)
	if err != nil {
		return nil, err
	}

	result = authorizeAndFilter(opt, result, func(d DiscoveryInfo) string {
		return d.RenovatorUID
	}, authorizedUIDs)

	result = util.EmptyIfNil(result)

	util.SortItems(
		result,
		util.SortBy(opt.SortBy),
		util.SortOrder(opt.Order),
		func(i DiscoveryInfo) string { return i.Name },
		func(i DiscoveryInfo) time.Time { return i.CreatedAt },
	)

	return result, nil
}

// PRActivitySummary is the per-Renovator aggregate of open PR activity
// derived from the most recent successful job's log output.
type PRActivitySummary struct {
	Open          int
	NeedsApproval int
	Unchanged     int
	HasRecentData bool
	WarnCount     int
	ErrorCount    int
}

// PerRepoActivity holds per-repo PR activity and warning counts consumed by
// the GitRepo list view.
type PerRepoActivity struct {
	OpenPRs       int
	NeedsApproval int
	Unchanged     int
	WarnCount     int
	ErrorCount    int
}

// GetPerRepoActivity returns per-repo PR activity and warning counts for each
// GitRepo of a Renovator. Results are cached briefly and de-duplicated via
// singleflight.
func (df *DataFactory) GetPerRepoActivity(
	ctx context.Context,
	opts ...ListOptions,
) (map[string]PerRepoActivity, error) {
	opt := getListOptions(opts)

	if opt.Namespace == "" || opt.Renovator == "" {
		return map[string]PerRepoActivity{}, nil
	}

	cacheKey := df.prActivityCacheKey(ctx, opt.Namespace, opt.Renovator)

	loaderCtx := context.WithoutCancel(ctx)

	result, err, _ := df.prActivityGroup.Do(cacheKey, func() (any, error) {
		loader := otter.LoaderFunc[string, map[string]PerRepoActivity](
			func(_ context.Context, _ string) (map[string]PerRepoActivity, error) {
				return df.computePerRepoActivity(loaderCtx, opt)
			},
		)

		return df.prActivityCache.Get(loaderCtx, cacheKey, loader)
	})
	if err != nil {
		return nil, err
	}

	perRepo, ok := result.(map[string]PerRepoActivity)
	if !ok {
		return nil, errUnexpectedCacheResult
	}

	return perRepo, nil
}

// GetPRActivityForRenovator aggregates open PR counts across every GitRepo
// of a Renovator by parsing the most recent completed job's log per repo.
// "Open" is Created+Updated+Unchanged (PRs that still exist on the platform);
// Automerged is excluded. NeedsApproval is reported separately for tinting.
// Result is cached briefly and de-duplicated via singleflight.
func (df *DataFactory) GetPRActivityForRenovator(
	ctx context.Context,
	opts ...ListOptions,
) (PRActivitySummary, error) {
	perRepo, err := df.GetPerRepoActivity(ctx, opts...)
	if err != nil {
		return PRActivitySummary{}, err
	}

	summary := PRActivitySummary{}

	for _, entry := range perRepo {
		summary.Open += entry.OpenPRs
		summary.NeedsApproval += entry.NeedsApproval
		summary.Unchanged += entry.Unchanged
		summary.WarnCount += entry.WarnCount
		summary.ErrorCount += entry.ErrorCount
		summary.HasRecentData = true
	}

	return summary, nil
}

// computePerRepoActivity lists jobs for the Renovator once, partitions by
// GitRepo in memory, then parses the most recent completed job per repo with
// bounded concurrency. Repos the current user cannot access are filtered out.
func (df *DataFactory) computePerRepoActivity(
	ctx context.Context,
	opt ListOptions,
) (map[string]PerRepoActivity, error) {
	result := make(map[string]PerRepoActivity)

	repos, err := df.GetGitRepos(ctx, opt)
	if err != nil {
		return result, fmt.Errorf("failed to list repos for PR activity: %w", err)
	}

	repos = df.ApplyAccessFilter(ctx, repos)

	if len(repos) == 0 {
		return result, nil
	}

	latestByRepo, err := df.findLatestTerminalJobsByRepo(ctx, opt.Namespace, opt.Renovator)
	if err != nil {
		return result, fmt.Errorf("failed to list jobs for PR activity: %w", err)
	}

	if len(latestByRepo) == 0 {
		return result, nil
	}

	results := make(chan prJobSample, len(latestByRepo))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(defaultPRActivityConcurrency)

	for _, repo := range repos {
		repoLabel, err := k8s.SanitizeLabel(repo.Name)
		if err != nil {
			continue
		}

		job, ok := latestByRepo[repoLabel]
		if !ok {
			continue
		}

		g.Go(func() error {
			sample := df.parseJobPRActivity(ctx, repo.Namespace, job)

			sample.repoLabel = repoLabel
			results <- sample

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return result, err
	}

	close(results)

	for sample := range results {
		if !sample.OK {
			continue
		}

		result[sample.repoLabel] = PerRepoActivity{
			OpenPRs:       sample.Open,
			NeedsApproval: sample.NeedsApproval,
			Unchanged:     sample.Unchanged,
			WarnCount:     sample.WarnCount,
			ErrorCount:    sample.ErrorCount,
		}
	}

	return result, nil
}

// prJobSample is the per-repo result of inspecting the most recent
// completed Job's log output.
type prJobSample struct {
	repoLabel     string
	Open          int
	NeedsApproval int
	Unchanged     int
	OK            bool
	WarnCount     int
	ErrorCount    int
}

// readJobLogStream reads the log stream and reports whether the kubelet
// had more lines than the requested tailLines. The caller passes
// tailLines > 0 only when the upstream logreader over-read by one
// line for this purpose; the helper trims the extra line and returns
// truncated=true. When tailLines <= 0, the full stream is returned
// and truncated is always false.
func readJobLogStream(stream io.Reader, tailLines int64) (string, bool, error) {
	content, err := io.ReadAll(stream)
	if err != nil {
		return "", false, err
	}

	if tailLines <= 0 {
		return string(content), false, nil
	}

	text := strings.TrimSuffix(string(content), "\n")
	if text == "" {
		return "", false, nil
	}

	lines := strings.Split(text, "\n")
	if int64(len(lines)) <= tailLines {
		return string(content), false, nil
	}

	trimmed := strings.Join(lines[:tailLines], "\n")

	return trimmed, true, nil
}

// findLatestTerminalJobsByRepo lists Jobs for a Renovator (paginated to
// avoid silent truncation on long histories) and returns the most recent
// terminal job (completed or failed) per GitRepo, keyed by repo name.
// Single Renovator-scoped list replaces N per-repo lists.
func (df *DataFactory) findLatestTerminalJobsByRepo(
	ctx context.Context,
	namespace, renovatorUID string,
) (map[string]*batchv1.Job, error) {
	const pageSize = 500

	var (
		latest = make(map[string]*batchv1.Job)
		cont   string
	)

	for {
		var page batchv1.JobList

		opts := []client.ListOption{
			client.InNamespace(namespace),
			client.MatchingLabels{renovatev1beta1.LabelRenovator: renovatorUID},
			client.Limit(pageSize),
		}
		if cont != "" {
			opts = append(opts, client.Continue(cont))
		}

		if err := df.client.List(ctx, &page, opts...); err != nil {
			return nil, fmt.Errorf("failed to list jobs: %w", err)
		}

		for i := range page.Items {
			job := &page.Items[i]
			if !isJobFinished(job) {
				continue
			}

			repoName := job.Labels[renovatev1beta1.LabelGitRepo]
			if repoName == "" {
				continue
			}

			if existing, ok := latest[repoName]; ok {
				if job.CreationTimestamp.After(existing.CreationTimestamp.Time) {
					latest[repoName] = job
				}
			} else {
				latest[repoName] = job
			}
		}

		if len(page.Items) < pageSize || page.GetContinue() == "" {
			break
		}

		cont = page.GetContinue()
	}

	return latest, nil
}

// parseJobPRActivity streams a Job's pod logs through parser.ParseLogs
// and returns the parsed open-PR counts. Line-by-line parsing keeps memory
// bounded by per-line allocations rather than the full log buffer.
func (df *DataFactory) parseJobPRActivity(
	ctx context.Context,
	namespace string,
	job *batchv1.Job,
) prJobSample {
	stream, err := df.GetJobLogs(ctx, namespace, job.Name, 0)
	if err != nil {
		frontendLog.Info("Job logs unavailable",
			"namespace", namespace, "job", job.Name, "error", err)

		return prJobSample{}
	}
	defer stream.Close()

	res, err := parser.ParseLogs(stream, -1)
	if err != nil {
		frontendLog.Error(err, "Failed to parse PR activity from job logs",
			"namespace", namespace, "job", job.Name)

		return prJobSample{}
	}

	activity := res.PRActivity

	return prJobSample{
		Open:          activity.Created + activity.Updated + activity.Unchanged,
		NeedsApproval: activity.NeedsApproval,
		Unchanged:     activity.Unchanged,
		OK:            true,
		WarnCount:     res.WarnCount,
		ErrorCount:    res.ErrorCount,
	}
}

// isJobFinished reports whether the given Job has reached a terminal state
// (successful completion or failure).
func isJobFinished(job *batchv1.Job) bool {
	if job.Status.CompletionTime != nil {
		return true
	}

	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
			return true
		}

		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

// GetJobsForRepo fetches jobs associated with a specific GitRepo.
func (df *DataFactory) GetJobsForRepo(
	ctx context.Context,
	repoName string,
	opts ...ListOptions,
) ([]viewmodel.JobInfo, error) {
	opt := getListOptions(opts)

	var jobList batchv1.JobList

	repoLabel, err := k8s.SanitizeLabel(repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to sanitize repo name: %w", err)
	}

	listOpts := []client.ListOption{
		client.MatchingLabels{renovatev1beta1.LabelGitRepo: repoLabel},
	}

	if opt.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(opt.Namespace))
	}

	if err := df.client.List(ctx, &jobList, listOpts...); err != nil {
		return nil, err
	}

	var result []viewmodel.JobInfo

	for _, job := range jobList.Items {
		status := viewmodel.StatusRunning

		if job.Status.CompletionTime != nil {
			status = viewmodel.StatusSucceeded
		}

		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
				status = viewmodel.StatusSucceeded
			}

			if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
				status = viewmodel.StatusFailed
			}
		}

		runnerName := job.Labels[renovatev1beta1.LabelAppInstance]

		result = append(result, viewmodel.JobInfo{
			Name:      job.Name,
			Namespace: job.Namespace,
			Runner:    runnerName,
			Status:    status,
			CreatedAt: job.CreationTimestamp.Time,
		})
	}

	result = util.EmptyIfNil(result)

	if opt.SortBy == "" {
		opt.SortBy = "date"
		opt.Order = "desc"
	}

	util.SortItems(
		result,
		util.SortBy(opt.SortBy),
		util.SortOrder(opt.Order),
		func(i viewmodel.JobInfo) string { return i.Name },
		func(i viewmodel.JobInfo) time.Time { return i.CreatedAt },
	)

	return result, nil
}

// IsJobRunning reports whether the given Kubernetes Job is still running.
// A job is considered running if it has not reached a terminal state (completed
// or permanently failed).
func (df *DataFactory) IsJobRunning(ctx context.Context, namespace, job string) bool {
	var k8sJob batchv1.Job
	if err := df.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: job}, &k8sJob); err != nil {
		return false
	}

	return !isJobFinished(&k8sJob)
}

// GetJobLogs fetches the log stream from the most recent Pod created by the
// specified Job. A positive tailLines asks the kubelet for only the last N
// lines; a value <= 0 returns the full retained log.
func (df *DataFactory) GetJobLogs(
	ctx context.Context, namespace, jobName string, tailLines int64,
) (io.ReadCloser, error) {
	if df.logReader == nil {
		return nil, errLogReaderNotConfigured
	}

	return df.logReader.ReadJobLogs(ctx, namespace, jobName, renovate.ContainerName, tailLines)
}

// getUserReposMap returns the user's accessible repo map, handling auth checks,
// session extraction, provider lookup, and cache/fetch logic.
func (df *DataFactory) getUserReposMap(ctx context.Context) (map[string]bool, error) {
	if df.authManager == nil {
		return nil, errAuthNotEnabled
	}

	if err := df.checkAuthReady(); err != nil {
		return nil, err
	}

	if !df.authManager.IsEnabled() {
		return nil, errAuthNotEnabled
	}

	session, ok := auth.GetSessionData(ctx, df.authManager.SessionManager())
	if !ok {
		return nil, errNotAuthenticated
	}

	if session.AccessToken == "" {
		return map[string]bool{}, nil
	}

	provider, ok := df.authManager.Get(session.Provider)
	if !ok {
		return map[string]bool{}, nil
	}

	cacheKey := df.deriveCacheKey(session)
	if cacheKey == "" {
		return nil, errUnableToDeriveCacheKey
	}

	// Create HTTP client with TokenSource once per session - this is the centralized approach
	client, err := df.createAuthClient(&session)
	if err != nil {
		return nil, err
	}

	return df.getUserRepos(ctx, provider, client, cacheKey)
}

// createAuthClient creates an HTTP client with automatic token injection from the session.
// The client is cached per session and reused across requests for connection pooling.
// Tokens are read fresh from the session on each request, avoiding stale token issues.
func (df *DataFactory) createAuthClient(session *auth.SessionData) (*http.Client, error) {
	// Generate cache key from session
	cacheKey := df.deriveCacheKey(*session)
	if cacheKey == "" {
		return nil, errUnableToDeriveCacheKey
	}

	// Check if we already have a cached client for this session
	if cached, found := df.httpClientCache.GetIfPresent(cacheKey); found {
		return cached, nil
	}

	// Create callback to read current session from the session store
	sessionFunc := func(ctx context.Context) *auth.SessionData {
		session, ok := auth.GetSessionData(ctx, df.authManager.SessionManager())
		if !ok {
			return nil
		}

		return &session
	}

	// Create HTTP client that reads tokens from session on each request
	httpClient := auth.NewAuthClient(sessionFunc)
	httpClient.Timeout = defaultHTTPClientTimeout

	// Cache the client for this session
	df.httpClientCache.Set(cacheKey, httpClient)

	return httpClient, nil
}

// ApplyAccessFilter filters repos by user access if auth is enabled, failing closed on error.
func (df *DataFactory) ApplyAccessFilter(
	ctx context.Context,
	repos []viewmodel.GitRepoInfo,
) []viewmodel.GitRepoInfo {
	userRepos, err := df.getUserReposMap(ctx)
	if err != nil && !errors.Is(err, errAuthNotEnabled) {
		frontendLog.Error(err, "Failed to fetch user repos")

		return []viewmodel.GitRepoInfo{}
	}

	if errors.Is(err, errAuthNotEnabled) {
		return repos
	}

	filtered := make([]viewmodel.GitRepoInfo, 0, len(repos))

	for _, repo := range repos {
		if userRepos[repo.FullName] {
			filtered = append(filtered, repo)
		}
	}

	return util.EmptyIfNil(filtered)
}

// IsUserRepo checks if a single repo is accessible by the current user.
// Uses the cached access list when available, falling back to a direct single-repo check on cache miss.
func (df *DataFactory) IsUserRepo(ctx context.Context, fullName string) bool {
	userRepos, err := df.getUserReposMap(ctx)
	if err != nil && !errors.Is(err, errAuthNotEnabled) {
		frontendLog.Error(err, "Failed to fetch user repos", "repo", fullName)

		return false
	}

	if errors.Is(err, errAuthNotEnabled) {
		return true
	}

	if len(userRepos) == 0 {
		return false
	}

	if accessible, ok := userRepos[fullName]; ok {
		return accessible
	}

	session, ok := auth.GetSessionData(ctx, df.authManager.SessionManager())
	if !ok {
		return false
	}

	provider, ok := df.authManager.Get(session.Provider)
	if !ok {
		return false
	}

	// Create HTTP client with TokenSource once per session - centralized approach
	client, err := df.createAuthClient(&session)
	if err != nil {
		frontendLog.Error(err, "Failed to create auth client", "repo", fullName)

		return false
	}

	accessible, err := provider.IsUserRepo(ctx, client, fullName)
	if err != nil {
		frontendLog.Error(err, "Failed to check user repo", "repo", fullName)

		return false
	}

	return accessible
}

// getUserRepos retrieves user repositories with deduplication and caching.
func (df *DataFactory) getUserRepos(
	ctx context.Context,
	provider auth.AuthProvider,
	client *http.Client,
	cacheKey string,
) (map[string]bool, error) {
	fetch := func() (map[string]bool, error) {
		return provider.GetUserRepos(ctx, client)
	}

	if cacheKey == "" {
		return fetch()
	}

	result, err, _ := df.accessGroup.Do(cacheKey, func() (any, error) {
		loader := otter.LoaderFunc[string, map[string]bool](func(_ context.Context, _ string) (map[string]bool, error) {
			repos, err := fetch()
			if err != nil {
				return nil, err
			}

			return repos, nil
		})

		return df.accessCache.Get(ctx, cacheKey, loader)
	})
	if err != nil {
		return nil, err
	}

	repos, ok := result.(map[string]bool)
	if !ok {
		return nil, errUnexpectedCacheResult
	}

	return repos, nil
}

func getListOptions(opts []ListOptions) ListOptions {
	if len(opts) > 0 {
		return opts[0]
	}

	return ListOptions{}
}

func extractRenovatorUID(labels map[string]string) string {
	if labels == nil {
		return ""
	}

	return labels[renovatev1beta1.LabelRenovator]
}

func (df *DataFactory) checkAuthReady() error {
	if df.authManager != nil && df.authManager.IsIntended() && !df.authManager.IsEnabled() {
		return errAuthNotReady
	}

	return nil
}
