package logreader

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
})
