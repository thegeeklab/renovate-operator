package logreader

import (
	"context"
	"errors"
	"fmt"
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
)

var _ = Describe("KubernetesReader", func() {
	var (
		ctx       context.Context
		clientset *fake.Clientset
		reader    *KubernetesReader
		namespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientset = fake.NewSimpleClientset()
		reader = NewKubernetesReader(clientset)
		namespace = "default"
	})

	It("streams the requested container's log", func() {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: namespace,
				Labels:    map[string]string{"job-name": "test-job"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "renovate", Image: "renovate/renovate"},
				},
			},
		}
		_, err := clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		stream, err := reader.ReadJobLogs(ctx, namespace, "test-job", "renovate")
		Expect(err).NotTo(HaveOccurred())
		Expect(stream).NotTo(BeNil())

		stream.Close()
	})

	It("returns a stream that can be read after ReadJobLogs returns", func() {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: namespace,
				Labels:    map[string]string{"job-name": "test-job"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "renovate", Image: "renovate/renovate"},
				},
			},
		}
		_, err := clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		stream, err := reader.ReadJobLogs(ctx, namespace, "test-job", "renovate")
		Expect(err).NotTo(HaveOccurred())

		defer stream.Close()

		_, err = io.ReadAll(stream)
		if err != nil {
			Expect(errors.Is(err, context.Canceled)).To(BeFalse())
			Expect(errors.Is(err, context.DeadlineExceeded)).To(BeFalse())
		}
	})

	It("returns an error when the kubelet cannot serve the log", func() {
		clientset.PrependReactor("get", "pods", func(_ testing.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("connection refused")
		})

		_, err := reader.ReadJobLogs(ctx, namespace, "test-job", "renovate")
		Expect(err).To(HaveOccurred())
	})

	It("returns ErrNoPodsForJob when no pods match the job", func() {
		_, err := reader.ReadJobLogs(ctx, namespace, "missing-job", "renovate")
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, ErrNoPodsForJob)).To(BeTrue())
	})

	It("returns ErrPodsNotReady when only pending pods exist for the job", func() {
		pending := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pending-pod",
				Namespace: namespace,
				Labels:    map[string]string{"job-name": "test-job"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "renovate"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodPending},
		}
		_, err := clientset.CoreV1().Pods(namespace).Create(ctx, pending, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = reader.ReadJobLogs(ctx, namespace, "test-job", "renovate")
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, ErrPodsNotReady)).To(BeTrue())
		Expect(errors.Is(err, ErrNoPodsForJob)).To(BeFalse())
	})
})

var _ = Describe("shouldRetry", func() {
	statusError := func(code int) error {
		return &api_errors.StatusError{
			ErrStatus: metav1.Status{Code: int32(code), Message: "boom"},
		}
	}

	DescribeTable(
		"classifies errors correctly",
		func(err error, want bool) {
			Expect(shouldRetry(err)).To(Equal(want))
		},
		Entry("nil", nil, false),
		Entry("context.Canceled", context.Canceled, false),
		Entry("context.DeadlineExceeded", context.DeadlineExceeded, false),
		Entry("wrapped context.DeadlineExceeded", fmt.Errorf("wrapped: %w", context.DeadlineExceeded), false),
		Entry("404 NotFound", api_errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "x"), false),
		Entry("400 BadRequest", api_errors.NewBadRequest("bad"), false),
		Entry("422 Invalid", api_errors.NewInvalid(schema.GroupKind{Kind: "Pod"}, "x", nil), false),
		Entry("500 ServerError", statusError(httpServerError), true),
		Entry("503 ServerError", statusError(httpServerError+3), true),
		Entry("generic network error", errors.New("connection refused"), true),
	)
})
