package gitrepo

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("GitRepo Controller", func() {
	var (
		ctx                context.Context
		reconciler         *Reconciler
		typeNamespacedName types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		reconciler = &Reconciler{
			Client:      k8sClient,
			Scheme:      k8sClient.Scheme(),
			ExternalURL: "https://renovate.example.com",
		}
	})

	Context("When reconciling via Labels", func() {
		const (
			repoName   = "test-gitrepo-label"
			configName = "test-config-label"
			labelValue = "renovator-01"
		)

		BeforeEach(func() {
			typeNamespacedName = types.NamespacedName{
				Name:      repoName,
				Namespace: "default",
			}

			config := &renovatev1beta1.RenovateConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configName,
					Namespace: "default",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: labelValue,
					},
				},
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type:     "gitea",
						Endpoint: "https://gitea.com",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "gitea-token"},
								Key:                  "token",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			resource := &renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      repoName,
					Namespace: "default",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: labelValue,
					},
				},
				Spec: renovatev1beta1.GitRepoSpec{
					Name: "org/repo",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      repoName,
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, resource)

			config := &renovatev1beta1.RenovateConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configName,
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, config)
		})

		It("should resolve RenovateConfig via labels and attempt reconciliation", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to fetch secret for provider token"))
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})

	It("should handle missing RenovateConfig resource gracefully", func() {
		mockClient := &mockErrorClient{Client: k8sClient}
		errorReconciler := &Reconciler{Client: mockClient, Scheme: k8sClient.Scheme()}

		result, err := errorReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "missing-config-repo", Namespace: "default"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))
	})

	It("should handle a GitRepo with no matching RenovateConfig gracefully", func() {
		unlabeledRepo := &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unlabeled-repo",
				Namespace: "default",
			},
			Spec: renovatev1beta1.GitRepoSpec{
				Name: "org/repo",
			},
		}
		Expect(k8sClient.Create(ctx, unlabeledRepo)).To(Succeed())

		defer func() {
			_ = k8sClient.Delete(ctx, unlabeledRepo)
		}()

		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "unlabeled-repo", Namespace: "default"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))
	})

	It("should handle missing GitRepo resource gracefully", func() {
		nonExistentName := types.NamespacedName{
			Name:      "non-existent-repo",
			Namespace: "default",
		}

		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: nonExistentName,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))
	})
})

type mockErrorClient struct {
	client.Client
}

func (m *mockErrorClient) Get(
	ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption,
) error {
	if _, ok := obj.(*renovatev1beta1.RenovateConfig); ok {
		return api_errors.NewNotFound(renovatev1beta1.GroupVersion.WithResource("renovateconfigs").GroupResource(), key.Name)
	}

	return m.Client.Get(ctx, key, obj, opts...)
}
