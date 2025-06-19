package jobscheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestJobScheduler_CreateRenovatorJobs(t *testing.T) {
	// Setup test data
	batches := []util.Batch{
		{Repositories: []string{"repo1", "repo2"}},
		{Repositories: []string{"repo3", "repo4"}},
	}

	// Create temporary batch config file
	batchData, err := json.Marshal(batches)
	if err != nil {
		t.Fatalf("Failed to marshal batch data: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "batches-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(batchData); err != nil {
		t.Fatalf("Failed to write batch data: %v", err)
	}
	tmpFile.Close()

	// Setup scheme
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add clientgo scheme: %v", err)
	}
	if err := renovatev1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add renovate scheme: %v", err)
	}

	// Create test renovator
	renovator := &renovatev1beta1.Renovator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-renovator",
			Namespace: "default",
		},
		Spec: renovatev1beta1.RenovatorSpec{
			Image: "renovate/renovate:latest",
			Renovate: renovatev1beta1.RenovateSpec{
				Image: "renovate/renovate:latest",
			},
			Runner: renovatev1beta1.RunnerSpec{
				Instances: 2,
			},
		},
	}

	// Create fake client
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(renovator).
		Build()

	// Create job scheduler
	js := &JobScheduler{
		RenovatorName:      "test-renovator",
		RenovatorNamespace: "default",
		BatchConfigFile:    tmpFile.Name(),
		MaxParallelJobs:    2,
		KubeClient:         client,
		Scheme:             scheme,
	}

	// Test creating renovator jobs
	ctx := context.Background()
	err = js.CreateRenovatorJobs(ctx)
	if err != nil {
		t.Fatalf("Failed to create renovator jobs: %v", err)
	}

	// Verify jobs were created
	jobList := &renovatev1beta1.RenovatorJobList{}
	err = client.List(ctx, jobList)
	if err != nil {
		t.Fatalf("Failed to list renovator jobs: %v", err)
	}

	if len(jobList.Items) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobList.Items))
	}

	// Verify job properties
	for i, job := range jobList.Items {
		if job.Spec.RenovatorName != "test-renovator" {
			t.Errorf("Job %d: expected renovator name 'test-renovator', got '%s'", i, job.Spec.RenovatorName)
		}

		if len(job.Spec.Repositories) != 2 {
			t.Errorf("Job %d: expected 2 repositories, got %d", i, len(job.Spec.Repositories))
		}

		if job.Labels["renovator.renovate/scheduled"] != "true" {
			t.Errorf("Job %d: expected scheduled label to be 'true'", i)
		}
	}
}

func TestJobScheduler_readBatchConfig(t *testing.T) {
	batches := []util.Batch{
		{Repositories: []string{"repo1", "repo2"}},
		{Repositories: []string{"repo3"}},
	}

	// Create temporary file
	batchData, err := json.Marshal(batches)
	if err != nil {
		t.Fatalf("Failed to marshal batch data: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "batches-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(batchData); err != nil {
		t.Fatalf("Failed to write batch data: %v", err)
	}
	tmpFile.Close()

	js := &JobScheduler{
		BatchConfigFile: tmpFile.Name(),
	}

	result, err := js.readBatchConfig()
	if err != nil {
		t.Fatalf("Failed to read batch config: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 batches, got %d", len(result))
	}

	if len(result[0].Repositories) != 2 {
		t.Errorf("Expected 2 repositories in first batch, got %d", len(result[0].Repositories))
	}

	if len(result[1].Repositories) != 1 {
		t.Errorf("Expected 1 repository in second batch, got %d", len(result[1].Repositories))
	}
}

func TestJobScheduler_countRunningJobs(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add clientgo scheme: %v", err)
	}
	if err := renovatev1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add renovate scheme: %v", err)
	}

	// Create test jobs
	runningJob := &renovatev1beta1.RenovatorJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "running-job",
			Namespace: "default",
			Labels: map[string]string{
				"renovator.renovate/name": "test-renovator",
			},
		},
		Status: renovatev1beta1.RenovatorJobStatus{
			Phase: renovatev1beta1.JobPhaseRunning,
		},
	}

	completedJob := &renovatev1beta1.RenovatorJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "completed-job",
			Namespace: "default",
			Labels: map[string]string{
				"renovator.renovate/name": "test-renovator",
			},
		},
		Status: renovatev1beta1.RenovatorJobStatus{
			Phase: renovatev1beta1.JobPhaseSucceeded,
		},
	}

	// Create fake client
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(runningJob, completedJob).
		Build()

	js := &JobScheduler{
		RenovatorName:      "test-renovator",
		RenovatorNamespace: "default",
		KubeClient:         client,
	}

	ctx := context.Background()
	count, err := js.countRunningJobs(ctx)
	if err != nil {
		t.Fatalf("Failed to count running jobs: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 running job, got %d", count)
	}
}

func TestJobScheduler_generateJobName(t *testing.T) {
	tests := []struct {
		name           string
		renovatorName  string
		batchIndex     int
		expectedMaxLen int
	}{
		{
			name:           "short renovator name",
			renovatorName:  "test-renovator",
			batchIndex:     0,
			expectedMaxLen: 50, // Should be well under limit
		},
		{
			name:           "long renovator name",
			renovatorName:  "theorigamicorporation-very-long-renovator-name-that-exceeds-limits",
			batchIndex:     9,
			expectedMaxLen: 50, // Should be truncated to fit
		},
		{
			name:           "medium renovator name",
			renovatorName:  "medium-length-renovator-name",
			batchIndex:     5,
			expectedMaxLen: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &JobScheduler{
				RenovatorName: tt.renovatorName,
			}

			jobName := js.generateJobName(tt.batchIndex)

			// Check length constraints
			if len(jobName) > tt.expectedMaxLen {
				t.Errorf("Generated job name '%s' is too long: %d chars (max %d)",
					jobName, len(jobName), tt.expectedMaxLen)
			}

			// Check that it contains the batch index
			expectedBatch := fmt.Sprintf("b%d", tt.batchIndex)
			if !strings.Contains(jobName, expectedBatch) {
				t.Errorf("Generated job name '%s' should contain batch identifier '%s'",
					jobName, expectedBatch)
			}

			// Check that the name is valid for Kubernetes (DNS-1123 subdomain)
			if !isValidKubernetesName(jobName) {
				t.Errorf("Generated job name '%s' is not a valid Kubernetes name", jobName)
			}

			// Simulate adding "-job" suffix (what RenovatorJob controller does)
			fullJobName := jobName + "-job"
			if len(fullJobName) > 63 {
				t.Errorf("Full job name '%s' exceeds Kubernetes limit: %d chars (max 63)",
					fullJobName, len(fullJobName))
			}
		})
	}
}

// isValidKubernetesName checks if a name is valid for Kubernetes resources
func isValidKubernetesName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}

	// Must start and end with alphanumeric
	if !isAlphaNumeric(name[0]) || !isAlphaNumeric(name[len(name)-1]) {
		return false
	}

	// Can contain alphanumeric and hyphens
	for _, r := range name {
		if !isAlphaNumeric(byte(r)) && r != '-' {
			return false
		}
	}

	return true
}

func isAlphaNumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}
