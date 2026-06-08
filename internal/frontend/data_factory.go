package frontend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/maypok86/otter/v2"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/auth"
	"github.com/thegeeklab/renovate-operator/internal/frontend/viewmodel"
	"github.com/thegeeklab/renovate-operator/pkg/util"
	"golang.org/x/sync/singleflight"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errPodNotFound            = errors.New("no pods found for job")
	errUnableToDeriveCacheKey = errors.New("unable to derive cache key for session")
	errUnexpectedCacheResult  = errors.New("unexpected cache result type")
	errAuthNotEnabled         = errors.New("auth not enabled")
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
	defaultAccessCacheTTL = 60 * time.Second
	defaultAccessCacheMax = 500
)

func hashAccessToken(token string) string {
	hash := sha256.Sum256([]byte(token))

	return hex.EncodeToString(hash[:])
}

func (df *DataFactory) deriveCacheKey(session auth.SessionData) string {
	if session.Subject != "" {
		return session.Provider + "|" + session.Subject
	}

	if session.AccessToken != "" {
		return session.Provider + "|token:" + hashAccessToken(session.AccessToken)
	}

	return ""
}

// DataFactory provides methods to fetch and transform data for both API and UI handlers.
type DataFactory struct {
	client      client.Client
	clientset   kubernetes.Interface
	authManager *auth.Manager
	accessCache *otter.Cache[string, map[string]bool]
	accessGroup singleflight.Group
}

// NewDataFactory creates a new DataFactory instance.
func NewDataFactory(client client.Client, clientset kubernetes.Interface, authManager *auth.Manager) *DataFactory {
	accessCache := otter.Must(&otter.Options[string, map[string]bool]{
		ExpiryCalculator: otter.ExpiryAccessing[string, map[string]bool](defaultAccessCacheTTL),
		MaximumSize:      defaultAccessCacheMax,
	})

	return &DataFactory{
		client:      client,
		clientset:   clientset,
		authManager: authManager,
		accessCache: accessCache,
	}
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

// GetRenovators fetches Renovator resources and transforms them into RenovatorInfo.
func (df *DataFactory) GetRenovators(ctx context.Context, opts ...ListOptions) ([]RenovatorInfo, error) {
	opt := getListOptions(opts)

	var listOpts []client.ListOption
	if opt.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(opt.Namespace))
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

	if result == nil {
		result = []RenovatorInfo{}
	}

	util.SortItems(
		result,
		util.SortBy(opt.SortBy),
		util.SortOrder(opt.Order),
		func(i RenovatorInfo) string { return i.Name },
		func(i RenovatorInfo) time.Time { return i.CreatedAt },
	)

	return result, nil
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
		lastStatus, lastTime := getRenovateStatusFromConditions(&gitrepo)

		result = append(result, viewmodel.GitRepoInfo{
			Name:               gitrepo.Name,
			FullName:           gitrepo.Spec.Name,
			Namespace:          gitrepo.Namespace,
			WebhookID:          gitrepo.Status.WebhookID,
			LastRenovateAt:     lastTime,
			LastRenovateStatus: lastStatus,
			CreatedAt:          gitrepo.CreationTimestamp.Time,
		})
	}

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

	if result == nil {
		result = []viewmodel.GitRepoInfo{}
	}

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
			Name:      runner.Name,
			Namespace: runner.Namespace,
			CreatedAt: runner.CreationTimestamp.Time,
		})
	}

	if result == nil {
		result = []RunnerInfo{}
	}

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
			Name:      discovery.Name,
			Namespace: discovery.Namespace,
			Schedule:  discovery.Spec.Schedule,
			CreatedAt: discovery.CreationTimestamp.Time,
		})
	}

	if result == nil {
		result = []DiscoveryInfo{}
	}

	util.SortItems(
		result,
		util.SortBy(opt.SortBy),
		util.SortOrder(opt.Order),
		func(i DiscoveryInfo) string { return i.Name },
		func(i DiscoveryInfo) time.Time { return i.CreatedAt },
	)

	return result, nil
}

// GetJobsForRepo fetches jobs associated with a specific GitRepo.
func (df *DataFactory) GetJobsForRepo(
	ctx context.Context,
	repoName string,
	opts ...ListOptions,
) ([]viewmodel.JobInfo, error) {
	opt := getListOptions(opts)

	var jobList batchv1.JobList

	listOpts := []client.ListOption{
		client.MatchingLabels{renovatev1beta1.LabelGitRepo: repoName},
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
			if cond.Type == batchv1.JobComplete && cond.Status == "True" {
				status = viewmodel.StatusSucceeded
			}

			if cond.Type == batchv1.JobFailed && cond.Status == "True" {
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

	if result == nil {
		result = []viewmodel.JobInfo{}
	}

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

// GetJobLogs fetches the log stream from the most recent Pod created by the specified Job.
func (df *DataFactory) GetJobLogs(ctx context.Context, namespace, jobName string) (io.ReadCloser, error) {
	podList, err := df.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for job %s: %w", jobName, err)
	}

	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("%w: %s", errPodNotFound, jobName)
	}

	slices.SortFunc(podList.Items, func(a, b corev1.Pod) int {
		return a.CreationTimestamp.Compare(b.CreationTimestamp.Time)
	})
	latestPod := podList.Items[len(podList.Items)-1]

	req := df.clientset.CoreV1().Pods(namespace).GetLogs(latestPod.Name, &corev1.PodLogOptions{})

	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open log stream for pod %s: %w", latestPod.Name, err)
	}

	return stream, nil
}

// getUserReposMap returns the user's accessible repo map, handling auth checks,
// session extraction, provider lookup, and cache/fetch logic.
func (df *DataFactory) getUserReposMap(ctx context.Context) (map[string]bool, error) {
	if df.authManager == nil || !df.authManager.IsEnabled() {
		return nil, errAuthNotEnabled
	}

	session, ok := auth.GetSessionData(ctx, df.authManager.Session)
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

	return df.getUserRepos(ctx, provider, session.AccessToken, cacheKey)
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

	if filtered == nil {
		filtered = []viewmodel.GitRepoInfo{}
	}

	return filtered
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

	session, ok := auth.GetSessionData(ctx, df.authManager.Session)
	if !ok {
		return false
	}

	provider, ok := df.authManager.Get(session.Provider)
	if !ok {
		return false
	}

	accessible, err := provider.IsUserRepo(ctx, session.AccessToken, fullName)
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
	token string,
	cacheKey string,
) (map[string]bool, error) {
	fetch := func() (map[string]bool, error) {
		return provider.GetUserRepos(ctx, token)
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
