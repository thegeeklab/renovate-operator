package jobscheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/dispatcher"
	"github.com/thegeeklab/renovate-operator/pkg/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	ErrInvalidConfig     = fmt.Errorf("invalid configuration")
	ErrKubernetesClient  = fmt.Errorf("failed to create kubernetes client")
	ErrJobCreation       = fmt.Errorf("failed to create renovator job")
	ErrRenovatorNotFound = fmt.Errorf("renovator instance not found")
)

type JobScheduler struct {
	RenovatorName      string
	RenovatorNamespace string
	BatchConfigFile    string
	MaxParallelJobs    int32
	KubeClient         client.Client
	Scheme             *runtime.Scheme
}

const (
	EnvRenovatorName      = "RENOVATOR_NAME"
	EnvRenovatorNamespace = "RENOVATOR_NAMESPACE"
	EnvBatchConfigFile    = "BATCH_CONFIG_FILE"
	EnvMaxParallelJobs    = "MAX_PARALLEL_JOBS"
)

func New() (*JobScheduler, error) {
	js := &JobScheduler{}

	var err error
	if js.RenovatorName, err = parseEnv(EnvRenovatorName); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	if js.RenovatorNamespace, err = parseEnv(EnvRenovatorNamespace); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	if js.BatchConfigFile, err = parseEnv(EnvBatchConfigFile); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	maxParallelStr, err := parseEnv(EnvMaxParallelJobs)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	maxParallel, err := strconv.ParseInt(maxParallelStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse max parallel jobs: %w", err)
	}
	js.MaxParallelJobs = int32(maxParallel)

	// Initialize Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKubernetesClient, err)
	}

	js.Scheme = runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(js.Scheme); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKubernetesClient, err)
	}
	if err := renovatev1beta1.AddToScheme(js.Scheme); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKubernetesClient, err)
	}
	if err := batchv1.AddToScheme(js.Scheme); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKubernetesClient, err)
	}
	if err := corev1.AddToScheme(js.Scheme); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKubernetesClient, err)
	}

	js.KubeClient, err = client.New(config, client.Options{Scheme: js.Scheme})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKubernetesClient, err)
	}

	return js, nil
}

func (js *JobScheduler) CreateRenovatorJobs(ctx context.Context) error {
	// Read batch configuration
	batches, err := js.readBatchConfig()
	if err != nil {
		return err
	}

	// Get the Renovator instance
	renovator, err := js.getRenovatorInstance(ctx)
	if err != nil {
		return err
	}

	// Check how many jobs are currently running
	runningJobs, err := js.countRunningJobs(ctx)
	if err != nil {
		return err
	}

	// Create RenovatorJobs for batches, respecting the parallel limit
	jobsToCreate := min(int(js.MaxParallelJobs-runningJobs), len(batches))
	if jobsToCreate <= 0 {
		return nil // No jobs to create
	}

	for i := 0; i < jobsToCreate; i++ {
		batch := batches[i]
		if err := js.createRenovatorJob(ctx, renovator, batch, i); err != nil {
			return fmt.Errorf("%w: batch %d: %w", ErrJobCreation, i, err)
		}
	}

	return nil
}

func (js *JobScheduler) readBatchConfig() ([]util.Batch, error) {
	data, err := os.ReadFile(js.BatchConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch config file: %w", err)
	}

	var batches []util.Batch
	if err := json.Unmarshal(data, &batches); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch config: %w", err)
	}

	return batches, nil
}

func (js *JobScheduler) getRenovatorInstance(ctx context.Context) (*renovatev1beta1.Renovator, error) {
	renovator := &renovatev1beta1.Renovator{}
	key := types.NamespacedName{
		Name:      js.RenovatorName,
		Namespace: js.RenovatorNamespace,
	}

	if err := js.KubeClient.Get(ctx, key, renovator); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRenovatorNotFound, err)
	}

	return renovator, nil
}

func (js *JobScheduler) countRunningJobs(ctx context.Context) (int32, error) {
	jobList := &renovatev1beta1.RenovatorJobList{}

	if err := js.KubeClient.List(ctx, jobList,
		client.InNamespace(js.RenovatorNamespace),
		client.MatchingLabels{"renovator.renovate/name": js.RenovatorName},
	); err != nil {
		return 0, fmt.Errorf("failed to list renovator jobs: %w", err)
	}

	var runningCount int32
	for _, job := range jobList.Items {
		if job.Status.Phase == renovatev1beta1.JobPhasePending ||
			job.Status.Phase == renovatev1beta1.JobPhaseRunning {
			runningCount++
		}
	}

	return runningCount, nil
}

func (js *JobScheduler) createRenovatorJob(ctx context.Context, renovator *renovatev1beta1.Renovator, batch util.Batch, batchIndex int) error {
	jobName := fmt.Sprintf("%s-scheduled-batch-%d-%d", js.RenovatorName, batchIndex, time.Now().Unix())

	// Create job spec for this batch
	jobSpec := js.createJobSpecForBatch(renovator, batchIndex)

	renovatorJob := &renovatev1beta1.RenovatorJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: js.RenovatorNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "renovate-operator",
				"app.kubernetes.io/name":       "renovator-job",
				"renovator.renovate/name":      js.RenovatorName,
				"renovator.renovate/scheduled": "true",
			},
		},
		Spec: renovatev1beta1.RenovatorJobSpec{
			RenovatorName: js.RenovatorName,
			Repositories:  batch.Repositories,
			JobSpec:       *jobSpec,
			BatchID:       fmt.Sprintf("scheduled-batch-%d", batchIndex),
			Priority:      int32(batchIndex),
		},
	}

	if err := controllerutil.SetControllerReference(renovator, renovatorJob, js.Scheme); err != nil {
		return err
	}

	return js.KubeClient.Create(ctx, renovatorJob)
}

func (js *JobScheduler) createJobSpecForBatch(renovator *renovatev1beta1.Renovator, batchIndex int) *batchv1.JobSpec {
	return &batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "renovate-operator",
					"app.kubernetes.io/name":       "renovator-runner",
					"renovator.renovate/scheduled": "true",
				},
			},
			Spec: js.createPodSpecForBatch(renovator, batchIndex),
		},
	}
}

func (js *JobScheduler) createPodSpecForBatch(renovator *renovatev1beta1.Renovator, batchIndex int) corev1.PodSpec {
	return corev1.PodSpec{
		ImagePullSecrets: renovator.Spec.ImagePullSecrets,
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
							Name: renovator.Name,
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
				Image:           renovator.Spec.Image,
				Command:         []string{"/dispatcher"},
				ImagePullPolicy: renovator.Spec.ImagePullPolicy,
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
			renovate.DefaultContainer(renovator, []corev1.EnvVar{}, []string{}),
		},
	}
}

func parseEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("environment variable %s is required", key)
	}
	return value, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
