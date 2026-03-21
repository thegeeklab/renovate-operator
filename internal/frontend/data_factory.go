package frontend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errPodNotFound = errors.New("no pods found for job")

// ListOptions holds optional parameters for filtering and sorting data.
type ListOptions struct {
	Namespace string
	Renovator string
	SortBy    string
	Order     string
}

// DataFactory provides methods to fetch and transform data for both API and UI handlers.
type DataFactory struct {
	client    client.Client
	clientset kubernetes.Interface
}

// NewDataFactory creates a new DataFactory instance.
func NewDataFactory(client client.Client, clientset kubernetes.Interface) *DataFactory {
	return &DataFactory{
		client:    client,
		clientset: clientset,
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
			Ready:     renovator.Status.Ready,
			CreatedAt: renovator.CreationTimestamp.Time,
		})
	}

	if result == nil {
		result = []RenovatorInfo{}
	}

	sortItems(result, opt,
		func(i RenovatorInfo) string { return i.Name },
		func(i RenovatorInfo) time.Time { return i.CreatedAt },
	)

	return result, nil
}

// GetGitRepos fetches GitRepo resources with optional filtering.
//
//nolint:dupl
func (df *DataFactory) GetGitRepos(ctx context.Context, opts ...ListOptions) ([]GitRepoInfo, error) {
	opt := getListOptions(opts)
	listOpts := buildListOptions(opt)

	var list renovatev1beta1.GitRepoList
	if err := df.client.List(ctx, &list, listOpts...); err != nil {
		return nil, err
	}

	var result []GitRepoInfo

	for _, gitrepo := range list.Items {
		result = append(result, GitRepoInfo{
			Name:      gitrepo.Name,
			Namespace: gitrepo.Namespace,
			WebhookID: gitrepo.Spec.WebhookID,
			Ready:     gitrepo.Status.Ready,
			CreatedAt: gitrepo.CreationTimestamp.Time,
		})
	}

	if result == nil {
		result = []GitRepoInfo{}
	}

	sortItems(result, opt,
		func(i GitRepoInfo) string { return i.Name },
		func(i GitRepoInfo) time.Time { return i.CreatedAt },
	)

	return result, nil
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
			Ready:     runner.Status.Ready,
			CreatedAt: runner.CreationTimestamp.Time,
		})
	}

	if result == nil {
		result = []RunnerInfo{}
	}

	sortItems(result, opt,
		func(i RunnerInfo) string { return i.Name },
		func(i RunnerInfo) time.Time { return i.CreatedAt },
	)

	return result, nil
}

// GetDiscoveries fetches Discovery resources with optional filtering.
//
//nolint:dupl
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
			Ready:     discovery.Status.Ready,
			CreatedAt: discovery.CreationTimestamp.Time,
		})
	}

	if result == nil {
		result = []DiscoveryInfo{}
	}

	sortItems(result, opt,
		func(i DiscoveryInfo) string { return i.Name },
		func(i DiscoveryInfo) time.Time { return i.CreatedAt },
	)

	return result, nil
}

// GetJobsForRepo fetches jobs associated with a specific GitRepo.
func (df *DataFactory) GetJobsForRepo(ctx context.Context, repoName string, opts ...ListOptions) ([]JobInfo, error) {
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

	var result []JobInfo

	for _, job := range jobList.Items {
		status := "Running"

		if job.Status.CompletionTime != nil {
			status = "Succeeded"
		}

		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == "True" {
				status = "Succeeded"
			}

			if cond.Type == batchv1.JobFailed && cond.Status == "True" {
				status = "Failed"
			}
		}

		runnerName := job.Labels[renovatev1beta1.LabelAppInstance]

		result = append(result, JobInfo{
			Name:      job.Name,
			Namespace: job.Namespace,
			Runner:    runnerName,
			Status:    status,
			CreatedAt: job.CreationTimestamp.Time,
		})
	}

	if result == nil {
		result = []JobInfo{}
	}

	if opt.SortBy == "" {
		opt.SortBy = "date"
		opt.Order = "desc"
	}

	sortItems(result, opt,
		func(i JobInfo) string { return i.Name },
		func(i JobInfo) time.Time { return i.CreatedAt },
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

	// Sort to find the most recent pod
	sort.Slice(podList.Items, func(i, j int) bool {
		return podList.Items[i].CreationTimestamp.Before(&podList.Items[j].CreationTimestamp)
	})
	latestPod := podList.Items[len(podList.Items)-1]

	// Request the log stream
	req := df.clientset.CoreV1().Pods(namespace).GetLogs(latestPod.Name, &corev1.PodLogOptions{
		// ⚡️ Removed Follow: true so io.ReadAll doesn't block forever
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open log stream for pod %s: %w", latestPod.Name, err)
	}

	return stream, nil
}

func getListOptions(opts []ListOptions) ListOptions {
	if len(opts) > 0 {
		return opts[0]
	}

	return ListOptions{}
}

func sortItems[T any](items []T, opt ListOptions, nameFn func(T) string, dateFn func(T) time.Time) {
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]

		var less, greater bool

		switch opt.SortBy {
		case "date":
			less = dateFn(a).Before(dateFn(b))
			greater = dateFn(a).After(dateFn(b))
		default:
			less = nameFn(a) < nameFn(b)
			greater = nameFn(a) > nameFn(b)
		}

		if opt.Order == "desc" {
			return greater
		}

		return less
	})
}
