package logstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var errPodNotFound = errors.New("no pods found for job")

// Manager provides a unified interface for retrieving logs.
type Manager struct {
	clientset kubernetes.Interface
	store     Store
}

// NewManager creates a hybrid log manager.
func NewManager(clientset kubernetes.Interface, store Store) *Manager {
	return &Manager{
		clientset: clientset,
		store:     store,
	}
}

// GetLogStream returns a log stream. If the job is currently running,
// it tails the active Pod in real-time. If the job is completed or deleted,
// it fetches the archived log from the persistent store.
func (m *Manager) GetLogStream(
	ctx context.Context, namespace, component, owner, jobName string,
) (io.ReadCloser, error) {
	job, err := m.clientset.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})

	isCurrentlyRunning := false
	if err == nil {
		isCurrentlyRunning = job.Status.Active > 0 && !scheduler.IsJobFinished(job)
	} else if !api_errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get job status: %w", err)
	}

	// If running, stream live from the Pod
	if isCurrentlyRunning {
		podList, err := m.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", jobName),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list pods for running job: %w", err)
		}

		if len(podList.Items) > 0 {
			sort.Slice(podList.Items, func(i, j int) bool {
				return podList.Items[i].CreationTimestamp.Before(&podList.Items[j].CreationTimestamp)
			})
			latestPod := podList.Items[len(podList.Items)-1]

			req := m.clientset.CoreV1().Pods(namespace).GetLogs(latestPod.Name, &corev1.PodLogOptions{})

			stream, err := req.Stream(ctx)
			if err == nil {
				return stream, nil
			}
		}
	}

	// Fallback to the static persistent archive (File/PVC or S3)
	return m.store.GetLog(ctx, namespace, component, owner, jobName)
}

// ArchiveJob finds the latest pod for a given job, streams its logs, and saves them to the persistent store.
// containerName can be empty if the pod only has one container.
func (m *Manager) ArchiveJob(ctx context.Context, namespace, jobName, component, owner, containerName string) error {
	podList, err := m.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods for job %s: %w", jobName, err)
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("%w: %s", errPodNotFound, jobName)
	}

	sort.Slice(podList.Items, func(i, j int) bool {
		return podList.Items[i].CreationTimestamp.Before(&podList.Items[j].CreationTimestamp)
	})
	latestPod := podList.Items[len(podList.Items)-1]

	logOptions := &corev1.PodLogOptions{}
	if containerName != "" {
		logOptions.Container = containerName
	}

	req := m.clientset.CoreV1().Pods(namespace).GetLogs(latestPod.Name, logOptions)

	podLogs, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("error opening log stream for pod %s: %w", latestPod.Name, err)
	}
	defer podLogs.Close()

	if err := m.store.SaveLog(ctx, namespace, component, owner, jobName, podLogs); err != nil {
		return fmt.Errorf("failed to save logs to persistent store: %w", err)
	}

	return nil
}

// ListLogs delegates directly to the persistent store.
// Running jobs won't show up here until they finish and are archived,
// which is the standard expected behavior for historical lists.
func (m *Manager) ListLogs(ctx context.Context, namespace, component, owner string) ([]LogEntry, error) {
	return m.store.ListLogs(ctx, namespace, component, owner)
}

// DeleteLog delegates directly to the persistent store.
func (m *Manager) DeleteLog(ctx context.Context, namespace, component, owner, jobName string) error {
	return m.store.DeleteLog(ctx, namespace, component, owner, jobName)
}

// Add this passthrough to Manager so you can use it directly if needed.
func (m *Manager) SaveLog(ctx context.Context, namespace, component, owner, jobName string, logReader io.Reader) error {
	return m.store.SaveLog(ctx, namespace, component, owner, jobName, logReader)
}
