package frontend

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DataFactory provides methods to fetch and transform data for both API and UI handlers.
type DataFactory struct {
	client client.Client
}

// NewDataFactory creates a new DataFactory instance.
func NewDataFactory(client client.Client) *DataFactory {
	return &DataFactory{
		client: client,
	}
}

// GetRenovators fetches Renovator resources and transforms them into RenovatorInfo.
func (df *DataFactory) GetRenovators(ctx context.Context) ([]RenovatorInfo, error) {
	var renovatorList renovatev1beta1.RenovatorList
	if err := df.client.List(ctx, &renovatorList); err != nil {
		return nil, err
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

	if result == nil {
		result = []RenovatorInfo{}
	}

	return result, nil
}

// GetGitRepos fetches GitRepo resources with optional filtering.
func (df *DataFactory) GetGitRepos(ctx context.Context, namespace, renovator string) ([]GitRepoInfo, error) {
	var gitRepoList renovatev1beta1.GitRepoList
	if err := df.client.List(ctx, &gitRepoList); err != nil {
		return nil, err
	}

	var result []GitRepoInfo

	for _, gitrepo := range gitRepoList.Items {
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

	if result == nil {
		result = []GitRepoInfo{}
	}

	return result, nil
}

// GetRunners fetches Runner resources with optional filtering.
func (df *DataFactory) GetRunners(ctx context.Context, namespace, renovator string) ([]RunnerInfo, error) {
	var runnerList renovatev1beta1.RunnerList
	if err := df.client.List(ctx, &runnerList); err != nil {
		return nil, err
	}

	var result []RunnerInfo

	for _, runner := range runnerList.Items {
		if (namespace != "" && runner.Namespace != namespace) ||
			(renovator != "" && runner.Namespace != renovator) {
			continue
		}

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

	return result, nil
}

// GetDiscoveries fetches Discovery resources with optional filtering.
func (df *DataFactory) GetDiscoveries(ctx context.Context, namespace, renovator string) ([]DiscoveryInfo, error) {
	var discoveryList renovatev1beta1.DiscoveryList
	if err := df.client.List(ctx, &discoveryList); err != nil {
		return nil, err
	}

	var result []DiscoveryInfo

	for _, discovery := range discoveryList.Items {
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

	if result == nil {
		result = []DiscoveryInfo{}
	}

	return result, nil
}

// GetJobsForRepo fetches jobs associated with a specific GitRepo.
func (df *DataFactory) GetJobsForRepo(ctx context.Context, namespace, repoName string) ([]JobInfo, error) {
	var jobList batchv1.JobList

	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{"renovate.thegeeklab.de/gitrepo": repoName},
	}

	if err := df.client.List(ctx, &jobList, opts...); err != nil {
		return nil, err
	}

	var result []JobInfo

	for _, job := range jobList.Items {
		status := "Running"
		if job.Status.Succeeded > 0 {
			status = "Succeeded"
		} else if job.Status.Failed > 0 {
			status = "Failed"
		}

		runnerName := job.Labels["app.kubernetes.io/instance"]

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

	return result, nil
}
