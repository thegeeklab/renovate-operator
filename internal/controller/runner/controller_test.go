package runner

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	v1beta1 "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Runner Controller", func() {
	var (
		ctx                context.Context
		reconciler         *Reconciler
		typeNamespacedName types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		reconciler = &Reconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	Context("When reconciling a resource via ConfigRef", func() {
		const resourceName = "test-runner-ref"

		BeforeEach(func() {
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			config := &renovatev1beta1.RenovateConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config-ref",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type: "github",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "token"},
								Key:                  "key",
							},
						},
						Endpoint: "https://api.github.com/",
					},
				},
			}
			rcd := &v1beta1.RenovateConfigCustomDefaulter{}
			Expect(rcd.Default(ctx, config)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			resource := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: renovatev1beta1.RunnerSpec{
					ConfigRef: "test-config-ref",
					JobSpec: renovatev1beta1.JobSpec{
						Schedule: "*/5 * * * *",
					},
				},
			}
			rd := &v1beta1.RunnerCustomDefaulter{}
			Expect(rd.Default(ctx, resource)).To(Succeed())
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			runner := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, runner)

			config := &renovatev1beta1.RenovateConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config-ref",
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, config)
		})

		It("should successfully reconcile the resource", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})
	})

	Context("When reconciling via Labels and handling GitRepo events", func() {
		const (
			runnerName = "test-runner-label"
			configName = "test-config-label"
			repoName   = "test-gitrepo"
			labelValue = "renovator-01"
		)

		BeforeEach(func() {
			config := &renovatev1beta1.RenovateConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configName,
					Namespace: "default",
					Labels: map[string]string{
						renovatev1beta1.RenovatorLabel: labelValue,
					},
				},
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type: "github",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "token"},
								Key:                  "key",
							},
						},
					},
				},
			}
			rcd := &v1beta1.RenovateConfigCustomDefaulter{}
			Expect(rcd.Default(ctx, config)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			runner := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      runnerName,
					Namespace: "default",
					Labels: map[string]string{
						renovatev1beta1.RenovatorLabel: labelValue,
					},
				},
				Spec: renovatev1beta1.RunnerSpec{
					JobSpec: renovatev1beta1.JobSpec{Schedule: "*/5 * * * *"},
				},
			}
			rd := &v1beta1.RunnerCustomDefaulter{}
			Expect(rd.Default(ctx, runner)).To(Succeed())
			Expect(k8sClient.Create(ctx, runner)).To(Succeed())
		})

		AfterEach(func() {
			runner := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      runnerName,
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, runner)

			config := &renovatev1beta1.RenovateConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configName,
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, config)
		})

		It("should resolve RenovateConfig via labels", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: runnerName, Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})

		It("should map GitRepo events to the correct Runner", func() {
			gitRepo := &renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      repoName,
					Namespace: "default",
					Labels: map[string]string{
						renovatev1beta1.RenovatorLabel: labelValue,
					},
				},
				Spec: renovatev1beta1.GitRepoSpec{
					Name: "test/repo",
				},
			}

			requests := reconciler.mapGitRepoToRunner(ctx, gitRepo)

			Expect(requests).To(HaveLen(1))
			Expect(requests[0].NamespacedName).To(Equal(types.NamespacedName{
				Name:      runnerName,
				Namespace: "default",
			}))
		})

		It("should NOT map GitRepo events if labels do not match", func() {
			gitRepo := &renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-repo",
					Namespace: "default",
					Labels: map[string]string{
						renovatev1beta1.RenovatorLabel: "wrong-id",
					},
				},
			}

			requests := reconciler.mapGitRepoToRunner(ctx, gitRepo)
			Expect(requests).To(BeEmpty())
		})
	})

	It("should handle missing RenovateConfig resource gracefully", func() {
		mockClient := &mockErrorClient{Client: k8sClient}
		errorReconciler := &Reconciler{Client: mockClient, Scheme: k8sClient.Scheme()}

		result, err := errorReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "missing-config-runner", Namespace: "default"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))
	})
})

// mockErrorClient is a mock client that returns errors for testing.
type mockErrorClient struct {
	client.Client
}

func (m *mockErrorClient) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	// Return error for RenovateConfig to simulate missing config
	if _, ok := obj.(*renovatev1beta1.RenovateConfig); ok {
		return api_errors.NewNotFound(renovatev1beta1.GroupVersion.WithResource("renovateconfigs").GroupResource(), key.Name)
	}

	return m.Client.Get(ctx, key, obj, opts...)
}
