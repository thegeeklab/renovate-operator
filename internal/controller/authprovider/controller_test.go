package authprovider

import (
	"context"
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/frontend/auth"
)

var _ = Describe("AuthProvider Controller", func() {
	var (
		ctx                context.Context
		reconciler         *Reconciler
		authManager        *auth.Manager
		typeNamespacedName types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()

		authManager = auth.NewManager(false)

		reconciler = &Reconciler{
			Client:        k8sClient,
			Scheme:        k8sClient.Scheme(),
			EventRecorder: &events.FakeRecorder{},
			AuthManager:   authManager,
		}
	})

	Context("When reconciling a resource", func() {
		const resourceName = "test-authprovider"

		BeforeEach(func() {
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"client-secret": []byte("test-client-secret"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			resource := &renovatev1beta1.AuthProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: renovatev1beta1.AuthProviderSpec{
					Type:      "gitea",
					Endpoint:  "https://gitea.example.com",
					IssuerURL: "https://gitea.example.com",
					ClientSecret: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-secret",
						},
						Key: "client-secret",
					},
					ClientID:    "test-client-id",
					RedirectURL: "https://operator.example.com/auth/callback",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &renovatev1beta1.AuthProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, resource)

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, secret)
		})

		It("should handle OIDC discovery failure gracefully", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			result, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create auth provider"))
			Expect(result.RequeueAfter).To(BeZero())

			_, ok := authManager.Get(resourceName)
			Expect(ok).To(BeFalse())
		})
	})

	Context("When the secret is missing", func() {
		const resourceName = "test-authprovider-missing-secret"

		BeforeEach(func() {
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			resource := &renovatev1beta1.AuthProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: renovatev1beta1.AuthProviderSpec{
					Type:     "gitea",
					Endpoint: "https://gitea.example.com",
					ClientSecret: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "non-existent-secret",
						},
						Key: "client-secret",
					},
					ClientID:    "test-client-id",
					RedirectURL: "https://operator.example.com/auth/callback",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &renovatev1beta1.AuthProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, resource)
		})

		It("should fail reconciliation", func() {
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When the secret is deleted after successful registration", func() {
		const resourceName = "test-authprovider-secret-deleted"

		BeforeEach(func() {
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			resource := &renovatev1beta1.AuthProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: renovatev1beta1.AuthProviderSpec{
					Type:     "gitea",
					Endpoint: "https://gitea.example.com",
					ClientSecret: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "non-existent-secret-deleted",
						},
						Key: "client-secret",
					},
					ClientID:    "test-client-id",
					RedirectURL: "https://operator.example.com/auth/callback",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &renovatev1beta1.AuthProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, resource)
		})

		It("should unregister the provider when secret is missing", func() {
			authManager.Register(&mockAuthProvider{name: resourceName})

			_, ok := authManager.Get(resourceName)
			Expect(ok).To(BeTrue())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).To(HaveOccurred())

			_, ok = authManager.Get(resourceName)
			Expect(ok).To(BeFalse())
		})
	})

	It("should handle missing AuthProvider resource gracefully", func() {
		nonExistentName := types.NamespacedName{
			Name:      "non-existent-authprovider",
			Namespace: "default",
		}

		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: nonExistentName,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))
	})
})

type mockAuthProvider struct {
	name string
}

func (m *mockAuthProvider) Type() string {
	return "mock"
}

func (m *mockAuthProvider) Name() string {
	return m.name
}

func (m *mockAuthProvider) LoginURL(_ string) string {
	return ""
}

func (m *mockAuthProvider) HandleCallback(_ context.Context, _ string) (*auth.AuthenticatedUser, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAuthProvider) RefreshToken(_ context.Context, _ string) (*auth.AuthenticatedUser, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAuthProvider) GetUserRepos(_ context.Context, _ *http.Client) (map[string]bool, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAuthProvider) IsUserRepo(_ context.Context, _ *http.Client, _ string) (bool, error) {
	return false, errors.New("not implemented")
}
