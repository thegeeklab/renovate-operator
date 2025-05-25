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

	// Create job spec template
	jobSpec := r.createJobSpecForRenovatorJob()

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
	cutoffTime := time.Now().Add(-1 * time.Hour)

	for _, job := range existingJobs.Items {
		// Only cleanup completed or failed jobs
		if job.Status.Phase == renovatev1beta1.JobPhaseSucceeded || job.Status.Phase == renovatev1beta1.JobPhaseFailed {
			if job.Status.CompletionTime != nil && job.Status.CompletionTime.Time.Before(cutoffTime) {
				if err := r.KubeClient.Delete(ctx, &job); err != nil && !errors.IsNotFound(err) {
					return err
				}
			}
		}
	}

	return nil
}

func (r *runnerReconciler) createJobSpecForRenovatorJob() *batchv1.JobSpec {
	return &batchv1.JobSpec{
		Template: r.createPodTemplateSpec(),
	}
}

func (r *runnerReconciler) createPodTemplateSpec() corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
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
