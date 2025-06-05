package runner

import (
	"context"
	"fmt"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/dispatcher"
	"github.com/thegeeklab/renovate-operator/pkg/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileRenovatorJobs creates and manages RenovatorJob CRs based on batches
func (r *runnerReconciler) reconcileRenovatorJobs(ctx context.Context) (*ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get existing RenovatorJobs for this Renovator instance
	existingJobs, err := r.listRenovatorJobs(ctx)
	if err != nil {
		return &ctrl.Result{}, err
	}

	// Check if we're suspended
	if r.instance.Spec.Suspend != nil && *r.instance.Spec.Suspend {
		// Delete all existing jobs if suspended
		for _, job := range existingJobs.Items {
			if err := r.KubeClient.Delete(ctx, &job); err != nil && !errors.IsNotFound(err) {
				return &ctrl.Result{}, err
			}
		}
		return &ctrl.Result{}, nil
	}

	// Count running jobs
	runningJobs := 0
	for _, job := range existingJobs.Items {
		if job.Status.Phase == renovatev1beta1.JobPhaseRunning || job.Status.Phase == renovatev1beta1.JobPhasePending {
			runningJobs++
		}
	}

	// Check if we need to create new jobs
	maxParallel := int(r.instance.Spec.Runner.Instances)
	if runningJobs >= maxParallel {
		logger.Info("Max parallel runners reached", "running", runningJobs, "max", maxParallel)
		return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Create RenovatorJobs for each batch
	for i, batch := range r.batches {
		if runningJobs >= maxParallel {
			break
		}

		jobName := fmt.Sprintf("%s-batch-%d-%d", r.instance.Name, i, time.Now().Unix())

		// Check if this batch already has a job
		if r.batchHasJob(existingJobs, batch) {
			continue
		}

		// Create job spec with batch index
		jobSpec := r.createJobSpecForRenovatorJob(i)

		renovatorJob := &renovatev1beta1.RenovatorJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: r.instance.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "renovate-operator",
					"app.kubernetes.io/name":       "renovator-job",
					"renovator.renovate/name":      r.instance.Name,
				},
			},
			Spec: renovatev1beta1.RenovatorJobSpec{
				RenovatorName: r.instance.Name,
				Repositories:  batch.Repositories,
				JobSpec:       *jobSpec,
				BatchID:       fmt.Sprintf("batch-%d", i),
				Priority:      int32(i), // Earlier batches have higher priority
			},
		}

		if err := controllerutil.SetControllerReference(r.instance, renovatorJob, r.Scheme); err != nil {
			return &ctrl.Result{}, err
		}

		if err := r.KubeClient.Create(ctx, renovatorJob); err != nil {
			logger.Error(err, "Failed to create RenovatorJob", "name", jobName)
			return &ctrl.Result{}, err
		}

		logger.Info("Created RenovatorJob", "name", jobName, "repositories", len(batch.Repositories))
		runningJobs++
	}

	// Clean up completed jobs older than 1 hour
	if err := r.cleanupOldJobs(ctx, existingJobs); err != nil {
		logger.Error(err, "Failed to cleanup old jobs")
		// Don't fail reconciliation on cleanup errors
	}

	return &ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (r *runnerReconciler) listRenovatorJobs(ctx context.Context) (*renovatev1beta1.RenovatorJobList, error) {
	jobList := &renovatev1beta1.RenovatorJobList{}

	labelSelector := labels.Set{
		"renovator.renovate/name": r.instance.Name,
	}

	if err := r.KubeClient.List(ctx, jobList,
		client.InNamespace(r.instance.Namespace),
		client.MatchingLabelsSelector{Selector: labelSelector.AsSelector()},
	); err != nil {
		return nil, err
	}

	return jobList, nil
}

func (r *runnerReconciler) batchHasJob(existingJobs *renovatev1beta1.RenovatorJobList, batch util.Batch) bool {
	for _, job := range existingJobs.Items {
		// Check if job is for this batch and is not completed/failed
		if job.Status.Phase != renovatev1beta1.JobPhaseSucceeded &&
			job.Status.Phase != renovatev1beta1.JobPhaseFailed {
			// Simple check: if repositories match
			if len(job.Spec.Repositories) == len(batch.Repositories) {
				match := true
				for i, repo := range job.Spec.Repositories {
					if i >= len(batch.Repositories) || repo != batch.Repositories[i] {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}
	return false
}

func (r *runnerReconciler) cleanupOldJobs(ctx context.Context, existingJobs *renovatev1beta1.RenovatorJobList) error {
	logger := log.FromContext(ctx)
	cutoffTime := time.Now().Add(-1 * time.Hour)
	cleanedJobs := 0

	for i := range existingJobs.Items {
		job := &existingJobs.Items[i]

		shouldCleanup := false
		reason := ""

		// Only cleanup completed or failed jobs
		if job.Status.Phase == renovatev1beta1.JobPhaseSucceeded || job.Status.Phase == renovatev1beta1.JobPhaseFailed {
			// Check if the job has completed and is older than the cutoff time
			if job.Status.CompletionTime != nil && job.Status.CompletionTime.Time.Before(cutoffTime) {
				shouldCleanup = true
				reason = "completed and older than cutoff time"
			} else if job.Status.CompletionTime == nil {
				// Fallback: check creation time for jobs without completion time
				if job.CreationTimestamp.Time.Before(cutoffTime.Add(-1 * time.Hour)) { // Extra hour buffer
					shouldCleanup = true
					reason = "old job without completion time"
				}
			}
		}

		if shouldCleanup {
			logger.Info("Cleaning up old job", "name", job.Name, "namespace", job.Namespace,
				"phase", job.Status.Phase,
				"completionTime", job.Status.CompletionTime,
				"creationTime", job.CreationTimestamp.Time,
				"reason", reason)

			if err := r.KubeClient.Delete(ctx, job); err != nil {
				if !errors.IsNotFound(err) {
					logger.Error(err, "Failed to delete old job", "name", job.Name)
					return err
				}
				// Job already deleted, just log it
				logger.Info("Job already deleted", "name", job.Name)
			} else {
				cleanedJobs++
			}
		} else {
			// Log why we're not cleaning up this job
			logger.V(4).Info("Not cleaning up job yet", "name", job.Name,
				"phase", job.Status.Phase,
				"completionTime", job.Status.CompletionTime,
				"creationTime", job.CreationTimestamp.Time,
				"cutoffTime", cutoffTime)
		}
	}

	if cleanedJobs > 0 {
		logger.Info("Cleaned up old jobs", "count", cleanedJobs)
	}

	return nil
}

func (r *runnerReconciler) createJobSpecForRenovatorJob(batchIndex int) *batchv1.JobSpec {
	jobSpec := &batchv1.JobSpec{
		Template: r.createPodTemplateSpec(batchIndex),
	}

	// Set TTL for automatic job cleanup (3600 seconds = 1 hour)
	ttl := int32(3600)
	jobSpec.TTLSecondsAfterFinished = &ttl

	return jobSpec
}

func (r *runnerReconciler) createPodTemplateSpec(batchIndex int) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "renovate-operator",
				"app.kubernetes.io/name":       "renovator-runner",
			},
		},
		Spec: corev1.PodSpec{
			ImagePullSecrets: r.instance.Spec.ImagePullSecrets,
			RestartPolicy:    corev1.RestartPolicyNever,
			Volumes: append(
				renovate.DefaultVolume(corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}),
				corev1.Volume{
					Name: renovate.VolumeRenovateTmp,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: r.instance.Name,
							},
						},
					},
				},
				corev1.Volume{
					Name: renovate.VolumeRenovateBase,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}),
			InitContainers: []corev1.Container{
				{
					Name:            "renovate-dispatcher",
					Image:           r.instance.Spec.Image,
					Command:         []string{"/dispatcher"},
					ImagePullPolicy: r.instance.Spec.ImagePullPolicy,
					Env: []corev1.EnvVar{
						{
							Name:  dispatcher.EnvRenovateRawConfig,
							Value: renovate.FileRenovateTmp,
						},
						{
							Name:  dispatcher.EnvRenovateConfig,
							Value: renovate.FileRenovateConfig,
						},
						{
							Name:  dispatcher.EnvRenovateBatches,
							Value: renovate.FileRenovateBatches,
						},
						{
							Name:  "JOB_COMPLETION_INDEX",
							Value: fmt.Sprintf("%d", batchIndex),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      renovate.VolumeRenovateConfig,
							MountPath: renovate.DirRenovateConfig,
						},
						{
							Name:      renovate.VolumeRenovateTmp,
							ReadOnly:  true,
							MountPath: renovate.DirRenovateTmp,
						},
					},
				},
			},
			Containers: []corev1.Container{
				renovate.DefaultContainer(r.instance, []corev1.EnvVar{}, []string{}),
			},
		},
	}
}

// RenovatorJobEqual checks if two RenovatorJob objects are equal
func RenovatorJobEqual(a, b client.Object) bool {
	ax, ok := a.(*renovatev1beta1.RenovatorJob)
	if !ok {
		return false
	}

	bx, ok := b.(*renovatev1beta1.RenovatorJob)
	if !ok {
		return false
	}

	// Compare relevant fields
	if ax.Spec.RenovatorName != bx.Spec.RenovatorName {
		return false
	}

	if len(ax.Spec.Repositories) != len(bx.Spec.Repositories) {
		return false
	}

	for i, repo := range ax.Spec.Repositories {
		if repo != bx.Spec.Repositories[i] {
			return false
		}
	}

	return equality.Semantic.DeepEqual(ax.Spec.JobSpec, bx.Spec.JobSpec)
}
