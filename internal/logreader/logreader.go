// Package logreader streams logs from a Kubernetes pod container.
package logreader

import (
	"context"
	"errors"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// httpServerError is the lower bound of HTTP status codes considered
// transient (5xx). 4xx errors (NotFound, BadRequest, etc.) are not retried.
const httpServerError = 500

// ErrNoPodsForJob is returned when a job has no associated pods at all.
var ErrNoPodsForJob = errors.New("no pods found for job")

// ErrPodsNotReady is returned when a job's pods exist but none have reached
// a phase (Running/Succeeded/Failed) that can serve logs.
var ErrPodsNotReady = errors.New("pods not ready for job")

// Reader streams logs from a job's latest pod.
// Implementations return the full retained log for the named container.
// The returned ReadCloser must be closed by the caller.
type Reader interface {
	ReadJobLogs(ctx context.Context, namespace, jobName, container string, tailLines int64) (io.ReadCloser, error)
}

// KubernetesReader implements Reader using the Kubernetes API.
type KubernetesReader struct {
	clientset kubernetes.Interface
}

var _ Reader = (*KubernetesReader)(nil)

// NewKubernetesReader returns a Reader backed by the given clientset.
func NewKubernetesReader(clientset kubernetes.Interface) *KubernetesReader {
	return &KubernetesReader{clientset: clientset}
}

func (r *KubernetesReader) readPodLogs(
	ctx context.Context, namespace, podName, container string, tailLines int64,
) (io.ReadCloser, error) {
	var stream io.ReadCloser

	err := retry.OnError(retry.DefaultRetry, shouldRetry, func() error {
		opts := &corev1.PodLogOptions{
			Container: container,
			Follow:    false,
		}

		if tailLines > 0 {
			// Over-read by one line so callers can detect truncation
			// without a second API call. The extra line is negligible
			// in transfer cost but lets us distinguish "exactly N
			// lines returned because the pod had exactly N" from
			// "exactly N lines returned because we asked for N".
			tail := tailLines + 1
			opts.TailLines = &tail
		}

		req := r.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)

		s, err := req.Stream(ctx)
		if err != nil {
			return err
		}

		stream = s

		return nil
	})
	if err != nil {
		return nil, err
	}

	return stream, nil
}

// ReadJobLogs resolves the pod for the given job and streams its container
// logs. It prefers a Succeeded pod; if none exist, it falls back to the most
// recent non-pending pod. Pods in the Pending phase are skipped because
// reading their logs would either fail or return no useful content.
//
// A positive tailLines asks the kubelet to return only the last N lines
// (server-side filter, no wasted transfer). A value <= 0 returns the full
// retained log.
func (r *KubernetesReader) ReadJobLogs(
	ctx context.Context, namespace, jobName, container string, tailLines int64,
) (io.ReadCloser, error) {
	podList, err := r.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return nil, fmt.Errorf("list pods for job %q: %w", jobName, err)
	}

	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoPodsForJob, jobName)
	}

	var (
		succeeded *corev1.Pod
		latest    *corev1.Pod
	)

	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Status.Phase == corev1.PodPending {
			continue
		}

		if pod.Status.Phase == corev1.PodSucceeded {
			if succeeded == nil || pod.CreationTimestamp.After(succeeded.CreationTimestamp.Time) {
				succeeded = pod
			}
		}

		if latest == nil || pod.CreationTimestamp.After(latest.CreationTimestamp.Time) {
			latest = pod
		}
	}

	target := latest
	if succeeded != nil {
		target = succeeded
	}

	if target == nil {
		return nil, fmt.Errorf("%w: %s", ErrPodsNotReady, jobName)
	}

	return r.readPodLogs(ctx, namespace, target.Name, container, tailLines)
}

// shouldRetry reports whether a Pods/GetLogs error is worth retrying.
// Context cancellation and 4xx (NotFound, BadRequest, Invalid) are terminal.
// 5xx and other transient errors retry.
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	if api_errors.IsNotFound(err) || api_errors.IsBadRequest(err) || api_errors.IsInvalid(err) {
		return false
	}

	var se *api_errors.StatusError
	if errors.As(err, &se) {
		return se.ErrStatus.Code >= httpServerError
	}

	return true
}
