package controller

import (
	"testing"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRenovatorJobReconciler_generateJobName(t *testing.T) {
	tests := []struct {
		name              string
		renovatorJobName  string
		expectedMaxLength int
	}{
		{
			name:              "short name",
			renovatorJobName:  "test-job",
			expectedMaxLength: 63,
		},
		{
			name:              "exactly at limit",
			renovatorJobName:  "this-is-exactly-fifty-eight-characters-long-name-test",
			expectedMaxLength: 63,
		},
		{
			name:              "exceeds limit",
			renovatorJobName:  "theorigamicorporation-renovator-scheduled-batch-9-1748149915",
			expectedMaxLength: 63,
		},
		{
			name:              "very long name",
			renovatorJobName:  "this-is-a-very-long-renovator-job-name-that-definitely-exceeds-the-kubernetes-limit-for-resource-names",
			expectedMaxLength: 63,
		},
	}

	r := &RenovatorJobReconciler{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobName := r.generateJobName(tt.renovatorJobName)

			if len(jobName) > tt.expectedMaxLength {
				t.Errorf("Generated job name '%s' is too long: %d chars (max %d)",
					jobName, len(jobName), tt.expectedMaxLength)
			}

			// Should always end with "-job"
			if len(jobName) < 4 || jobName[len(jobName)-4:] != "-job" {
				t.Errorf("Generated job name '%s' should end with '-job'", jobName)
			}

			// Calculate expected prefix based on the actual truncation logic
			jobSuffix := "-job"
			maxLength := 63 - len(jobSuffix)
			expectedPrefix := tt.renovatorJobName
			if len(expectedPrefix) > maxLength {
				expectedPrefix = expectedPrefix[:maxLength]
			}
			
			actualPrefix := jobName[:len(jobName)-4] // Remove "-job" suffix
			if actualPrefix != expectedPrefix {
				t.Errorf("Expected prefix '%s', got '%s'", expectedPrefix, actualPrefix)
			}
		})
	}
}

func TestRenovatorJobReconciler_createJob(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = renovatev1beta1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &RenovatorJobReconciler{
		Client: client,
		Scheme: scheme,
	}

	// Test with a long RenovatorJob name
	renovatorJob := &renovatev1beta1.RenovatorJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "theorigamicorporation-renovator-scheduled-batch-9-1748149915",
			Namespace: "default",
		},
		Spec: renovatev1beta1.RenovatorJobSpec{
			RenovatorName: "theorigamicorporation-renovator",
			Repositories:  []string{"repo1", "repo2"},
			JobSpec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Containers: []corev1.Container{
							{
								Name:  "renovate",
								Image: "renovate/renovate:latest",
							},
						},
					},
				},
			},
		},
	}

	job := r.createJob(renovatorJob)

	// Verify job name is within limits
	if len(job.Name) > 63 {
		t.Errorf("Job name '%s' exceeds Kubernetes limit: %d chars (max 63)",
			job.Name, len(job.Name))
	}

	// Verify labels are within limits
	for key, value := range job.Labels {
		if len(value) > 63 {
			t.Errorf("Label '%s' value '%s' exceeds Kubernetes limit: %d chars (max 63)",
				key, value, len(value))
		}
	}

	// Verify the job has the correct structure
	if job.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got '%s'", job.Namespace)
	}

	if job.Labels["app.kubernetes.io/managed-by"] != "renovate-operator" {
		t.Errorf("Expected managed-by label to be 'renovate-operator'")
	}

	if job.Labels["renovatorjob.renovate/name"] != "theorigamicorporation-renovator" {
		t.Errorf("Expected renovatorjob.renovate/name label to be 'theorigamicorporation-renovator'")
	}
}
