package runner

import (
	"context"
	"testing"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/reconciler"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileCronJob(t *testing.T) {
	tests := []struct {
		name              string
		renovatorSchedule string
		expectCronJob     bool
	}{
		{
			name:              "with schedule",
			renovatorSchedule: "0 2 * * *",
			expectCronJob:     true,
		},
		{
			name:              "without schedule",
			renovatorSchedule: "",
			expectCronJob:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			scheme := runtime.NewScheme()
			_ = renovatev1beta1.AddToScheme(scheme)
			_ = batchv1.AddToScheme(scheme)

			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Schedule: tt.renovatorSchedule,
					Renovate: renovatev1beta1.RenovateSpec{
						Image: "renovate/renovate:latest",
					},
				},
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(renovator).
				Build()

			r := &runnerReconciler{
				GenericReconciler: &reconciler.GenericReconciler{
					KubeClient: client,
					Scheme:     scheme,
					Req: ctrl.Request{
						NamespacedName: types.NamespacedName{
							Name:      "test-renovator",
							Namespace: "default",
						},
					},
				},
				instance: renovator,
			}

			// Execute
			_, err := r.reconcileCronJob(context.Background())
			if err != nil {
				t.Fatalf("reconcileCronJob() error = %v", err)
			}

			// Verify
			cronJob := &batchv1.CronJob{}
			err = client.Get(context.Background(), types.NamespacedName{
				Name:      "test-renovator-runner",
				Namespace: "default",
			}, cronJob)

			if tt.expectCronJob {
				if err != nil {
					t.Errorf("Expected CronJob to be created, but got error: %v", err)
				}
				if cronJob.Spec.Schedule != tt.renovatorSchedule {
					t.Errorf("Expected schedule %s, got %s", tt.renovatorSchedule, cronJob.Spec.Schedule)
				}
			} else {
				if err == nil {
					t.Errorf("Expected no CronJob to be created, but found one")
				}
			}
		})
	}
}
