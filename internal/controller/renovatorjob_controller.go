package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
)

// RenovatorJobReconciler reconciles a RenovatorJob object
type RenovatorJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovatorjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovatorjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=renovatorjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *RenovatorJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the RenovatorJob instance
	renovatorJob := &renovatev1beta1.RenovatorJob{}
	err := r.Get(ctx, req.NamespacedName, renovatorJob)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, could have been deleted
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Add finalizer for cleanup
	if !controllerutil.ContainsFinalizer(renovatorJob, "renovatorjob.finalizers.renovate.thegeeklab.de") {
		controllerutil.AddFinalizer(renovatorJob, "renovatorjob.finalizers.renovate.thegeeklab.de")
		if err := r.Update(ctx, renovatorJob); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Handle deletion
	if !renovatorJob.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, renovatorJob)
	}

	// Check if we already have a job reference
	if renovatorJob.Status.JobRef != nil {
		// Check the status of the existing job
		job := &batchv1.Job{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      renovatorJob.Status.JobRef.Name,
			Namespace: renovatorJob.Namespace,
		}, job)
		if err != nil {
			if errors.IsNotFound(err) {
				// Job was deleted, update status
				renovatorJob.Status.Phase = renovatev1beta1.JobPhaseFailed
				renovatorJob.Status.Message = "Job was deleted"
				if err := r.Status().Update(ctx, renovatorJob); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}

		// Update status based on job status
		return r.updateStatusFromJob(ctx, renovatorJob, job)
	}

	// Create the job if it doesn't exist
	logger.Info("Creating job for RenovatorJob", "name", renovatorJob.Name)

	job := r.createJob(renovatorJob)

	if err := controllerutil.SetControllerReference(renovatorJob, job, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, job); err != nil {
		logger.Error(err, "Failed to create Job", "name", job.Name)
		return ctrl.Result{}, err
	}

	// Update status with job reference
	renovatorJob.Status.JobRef = &corev1.LocalObjectReference{
		Name: job.Name,
	}
	renovatorJob.Status.Phase = renovatev1beta1.JobPhasePending
	now := metav1.NewTime(time.Now())
	renovatorJob.Status.StartTime = &now

	if err := r.Status().Update(ctx, renovatorJob); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *RenovatorJobReconciler) createJob(renovatorJob *renovatev1beta1.RenovatorJob) *batchv1.Job {
	// Create job spec based on the RenovatorJob spec
	jobSpec := renovatorJob.Spec.JobSpec.DeepCopy()

	// Add environment variables for repositories
	if len(jobSpec.Template.Spec.Containers) > 0 {
		container := &jobSpec.Template.Spec.Containers[0]

		// Add repository list as environment variable
		repoList := ""
		for i, repo := range renovatorJob.Spec.Repositories {
			if i > 0 {
				repoList += ","
			}
			repoList += repo
		}

		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "RENOVATE_REPOSITORIES",
			Value: repoList,
		})

		// Add batch ID if present
		if renovatorJob.Spec.BatchID != "" {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "RENOVATE_BATCH_ID",
				Value: renovatorJob.Spec.BatchID,
			})
		}
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-job", renovatorJob.Name),
			Namespace: renovatorJob.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "renovate-operator",
				"app.kubernetes.io/name":       "renovator-job",
				"app.kubernetes.io/instance":   renovatorJob.Name,
				"renovatorjob.renovate/name":   renovatorJob.Name,
			},
		},
		Spec: *jobSpec,
	}

	return job
}

func (r *RenovatorJobReconciler) updateStatusFromJob(ctx context.Context, renovatorJob *renovatev1beta1.RenovatorJob, job *batchv1.Job) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check job conditions
	for _, condition := range job.Status.Conditions {
		switch condition.Type {
		case batchv1.JobComplete:
			if condition.Status == corev1.ConditionTrue {
				renovatorJob.Status.Phase = renovatev1beta1.JobPhaseSucceeded
				renovatorJob.Status.CompletionTime = job.Status.CompletionTime
				renovatorJob.Status.Message = "Job completed successfully"

				// Mark all repositories as processed (in real implementation,
				// this would be updated based on actual processing results)
				renovatorJob.Status.ProcessedRepositories = renovatorJob.Spec.Repositories
			}
		case batchv1.JobFailed:
			if condition.Status == corev1.ConditionTrue {
				renovatorJob.Status.Phase = renovatev1beta1.JobPhaseFailed
				renovatorJob.Status.CompletionTime = job.Status.CompletionTime
				renovatorJob.Status.Message = condition.Message
			}
		}
	}

	// If job is still running
	if job.Status.Active > 0 {
		renovatorJob.Status.Phase = renovatev1beta1.JobPhaseRunning
		renovatorJob.Status.Message = fmt.Sprintf("Job is running with %d active pods", job.Status.Active)
	}

	// Update status
	if err := r.Status().Update(ctx, renovatorJob); err != nil {
		logger.Error(err, "Failed to update RenovatorJob status")
		return ctrl.Result{}, err
	}

	// Requeue if job is still running
	if renovatorJob.Status.Phase == renovatev1beta1.JobPhaseRunning || renovatorJob.Status.Phase == renovatev1beta1.JobPhasePending {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *RenovatorJobReconciler) finalize(ctx context.Context, renovatorJob *renovatev1beta1.RenovatorJob) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Clean up any resources if needed
	if renovatorJob.Status.JobRef != nil {
		job := &batchv1.Job{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      renovatorJob.Status.JobRef.Name,
			Namespace: renovatorJob.Namespace,
		}, job)

		if err == nil {
			// Delete the job
			propagationPolicy := metav1.DeletePropagationBackground
			if err := r.Delete(ctx, job, &client.DeleteOptions{
				PropagationPolicy: &propagationPolicy,
			}); err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "Failed to delete Job", "name", job.Name)
				return ctrl.Result{}, err
			}
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(renovatorJob, "renovatorjob.finalizers.renovate.thegeeklab.de")
	if err := r.Update(ctx, renovatorJob); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RenovatorJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.RenovatorJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
