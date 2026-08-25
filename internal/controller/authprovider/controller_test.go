package authprovider

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"time"

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
				Name:      "test-secret",
				Namespace: "default",
				Data: map[string][]byte{
					"client-secret": []byte("test-client-secret"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			resource := &renovatev1beta1.AuthProvider{
				Name:      resourceName,
				Namespace: "default",
				Spec: renovatev1beta1.AuthProviderSpec{
					Type:     "gitea",
					Endpoint: "https://gitea.example.com",
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
				Name:      resourceName,
				Namespace: "default",
			}
			_ = k8sClient.Delete(ctx, resource)

			secret := &corev1.Secret{
				Name:      "test-secret",
				Namespace: "default",
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
				Name:      resourceName,
				Namespace: "default",
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
				Name:      resourceName,
				Namespace: "default",
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
				Name:      resourceName,
				Namespace: "default",
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
				Name:      resourceName,
				Namespace: "default",
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

	It("should create a GitLab auth provider with configured values", func() {
		provider, err := reconciler.createAuthProvider(ctx, &renovatev1beta1.AuthProvider{
			Name: "gitlab-auth",
			Spec: renovatev1beta1.AuthProviderSpec{
				Type:        renovatev1beta1.PlatformType_GITLAB,
				Endpoint:    "https://gitlab.example.com",
				ForgeURL:    "https://gitlab.example.com/api/v4",
				AuthURL:     "https://login.example.com/authorize",
				DisplayName: "Company GitLab",
				IconURL:     "https://example.com/gitlab.svg",
				ClientID:    "client-id",
				RedirectURL: "https://operator.example.com/auth/callback",
				Insecure:    true,
			},
		}, "client-secret", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(provider.Type()).To(Equal(auth.ProviderTypeGitLab))
		Expect(provider.Name()).To(Equal("gitlab-auth"))
		Expect(provider.DisplayName()).To(Equal("Company GitLab"))
		Expect(provider.IconURL()).To(Equal("https://example.com/gitlab.svg"))
	})

	It("should reject an unknown auth provider type", func() {
		provider, err := reconciler.createAuthProvider(ctx, &renovatev1beta1.AuthProvider{
			Spec: renovatev1beta1.AuthProviderSpec{Type: renovatev1beta1.PlatformType("unknown")},
		}, "client-secret", nil)
		Expect(err).To(MatchError(ContainSubstring("unsupported platform type")))
		Expect(provider).To(BeNil())
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

	Context("When a CA bundle secret is referenced but missing", func() {
		const resourceName = "test-authprovider-missing-ca-secret"

		BeforeEach(func() {
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			secret := &corev1.Secret{
				Name:      "test-secret-ca",
				Namespace: "default",
				Data: map[string][]byte{
					"client-secret": []byte("test-client-secret"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			resource := &renovatev1beta1.AuthProvider{
				Name:      resourceName,
				Namespace: "default",
				Spec: renovatev1beta1.AuthProviderSpec{
					Type:     "gitea",
					Endpoint: "https://gitea.example.com",
					ClientSecret: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-secret-ca",
						},
						Key: "client-secret",
					},
					ClientID:    "test-client-id",
					RedirectURL: "https://operator.example.com/auth/callback",
					CABundleSecret: &corev1.SecretKeySelector{
						Name: "non-existent-ca-secret",
						Key:  "ca-bundle",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &renovatev1beta1.AuthProvider{
				Name:      resourceName,
				Namespace: "default",
			}
			_ = k8sClient.Delete(ctx, resource)

			secret := &corev1.Secret{
				Name:      "test-secret-ca",
				Namespace: "default",
			}
			_ = k8sClient.Delete(ctx, secret)
		})

		It("should fail reconciliation with CA bundle secret not found", func() {
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get CA bundle secret"))
		})
	})

	Context("getCABundleSecret", func() {
		var (
			secret       *corev1.Secret
			resourceName string
		)

		BeforeEach(func() {
			resourceName = "test-ca-getter"

			secret = &corev1.Secret{
				Name:      "ca-secret",
				Namespace: "default",
				Data: map[string][]byte{
					"ca.crt": []byte("ca-data"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, &corev1.Secret{
				Name:      "ca-secret",
				Namespace: "default",
			})
		})

		It("should return the secret when CABundleSecret is set", func() {
			ap := &renovatev1beta1.AuthProvider{
				Name: resourceName, Namespace: "default",
				Spec: renovatev1beta1.AuthProviderSpec{
					CABundleSecret: &corev1.SecretKeySelector{
						Name: "ca-secret",
						Key:  "ca.crt",
					},
				},
			}

			result, err := reconciler.getCABundleSecret(ctx, ap)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Name).To(Equal("ca-secret"))
		})

		It("should return an error when secret does not exist", func() {
			ap := &renovatev1beta1.AuthProvider{
				Name: resourceName, Namespace: "default",
				Spec: renovatev1beta1.AuthProviderSpec{
					CABundleSecret: &corev1.SecretKeySelector{
						Name: "non-existent",
						Key:  "ca.crt",
					},
				},
			}

			_, err := reconciler.getCABundleSecret(ctx, ap)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get CA bundle secret"))
		})
	})

	Context("extractCABundle", func() {
		var validPEM []byte

		BeforeEach(func() {
			caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			Expect(err).NotTo(HaveOccurred())

			caTmpl := &x509.Certificate{
				SerialNumber:          big.NewInt(1),
				Subject:               pkix.Name{CommonName: "test-ca"},
				NotBefore:             time.Now().Add(-time.Hour),
				NotAfter:              time.Now().Add(time.Hour),
				IsCA:                  true,
				BasicConstraintsValid: true,
				KeyUsage:              x509.KeyUsageCertSign,
			}

			caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
			Expect(err).NotTo(HaveOccurred())

			validPEM = pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: caDER,
			})
		})

		It("should extract valid PEM certificate data", func() {
			secret := &corev1.Secret{
				Data: map[string][]byte{
					"ca.crt": validPEM,
				},
			}

			ap := &renovatev1beta1.AuthProvider{
				Spec: renovatev1beta1.AuthProviderSpec{
					CABundleSecret: &corev1.SecretKeySelector{
						Key: "ca.crt",
					},
				},
			}

			result, err := reconciler.extractCABundle(ap, secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(validPEM))
		})

		It("should reject invalid PEM data", func() {
			secret := &corev1.Secret{
				Name: "ca-secret",
				Data: map[string][]byte{
					"ca.crt": []byte("not-valid-pem"),
				},
			}

			ap := &renovatev1beta1.AuthProvider{
				Spec: renovatev1beta1.AuthProviderSpec{
					CABundleSecret: &corev1.SecretKeySelector{
						Key: "ca.crt",
					},
				},
			}

			_, err := reconciler.extractCABundle(ap, secret)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no valid CA certificates found"))
		})

		It("should default key to ca.crt when key is empty", func() {
			secret := &corev1.Secret{
				Data: map[string][]byte{
					"ca.crt": validPEM,
				},
			}

			ap := &renovatev1beta1.AuthProvider{
				Spec: renovatev1beta1.AuthProviderSpec{
					CABundleSecret: &corev1.SecretKeySelector{
						Key: "",
					},
				},
			}

			result, err := reconciler.extractCABundle(ap, secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(validPEM))
		})

		It("should return an error when the key is missing from the secret", func() {
			secret := &corev1.Secret{
				Name: "ca-secret",
				Data: map[string][]byte{},
			}

			ap := &renovatev1beta1.AuthProvider{
				Spec: renovatev1beta1.AuthProviderSpec{
					CABundleSecret: &corev1.SecretKeySelector{
						Key: "ca.crt",
					},
				},
			}

			_, err := reconciler.extractCABundle(ap, secret)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("secret key not found"))
		})
	})

	Context("isProviderUpToDate", func() {
		It("should return false when CABundleSecretResourceVersion does not match", func() {
			ap := &renovatev1beta1.AuthProvider{
				Generation: 1,
				Status: renovatev1beta1.AuthProviderStatus{
					Registered:                    true,
					SecretResourceVersion:         "v1",
					CABundleSecretResourceVersion: "v1",
				},
			}
			ap.SetCondition(renovatev1beta1.ConditionReady, metav1.ConditionTrue, "Ready", "")

			authManager.Register(&mockAuthProvider{name: ap.Name})

			secret := &corev1.Secret{}
			secret.ResourceVersion = "v1"

			caSecret := &corev1.Secret{}
			caSecret.ResourceVersion = "v2"

			Expect(reconciler.isProviderUpToDate(ap, secret, caSecret)).To(BeFalse())
		})

		It("should return true when both secret versions match", func() {
			ap := &renovatev1beta1.AuthProvider{
				Name:       "test-provider-uptodate",
				Generation: 1,
				Status: renovatev1beta1.AuthProviderStatus{
					Registered:                    true,
					SecretResourceVersion:         "v1",
					CABundleSecretResourceVersion: "v1",
				},
			}
			ap.SetCondition(renovatev1beta1.ConditionReady, metav1.ConditionTrue, "Ready", "")

			authManager.Register(&mockAuthProvider{name: ap.Name})

			secret := &corev1.Secret{}
			secret.ResourceVersion = "v1"

			caSecret := &corev1.Secret{}
			caSecret.ResourceVersion = "v1"

			Expect(reconciler.isProviderUpToDate(ap, secret, caSecret)).To(BeTrue())
		})

		It("should return true when caSecret is nil and no CA bundle is configured", func() {
			ap := &renovatev1beta1.AuthProvider{
				Name:       "test-provider-no-ca",
				Generation: 1,
				Status: renovatev1beta1.AuthProviderStatus{
					Registered:            true,
					SecretResourceVersion: "v1",
				},
			}
			ap.SetCondition(renovatev1beta1.ConditionReady, metav1.ConditionTrue, "Ready", "")

			authManager.Register(&mockAuthProvider{name: ap.Name})

			secret := &corev1.Secret{}
			secret.ResourceVersion = "v1"

			Expect(reconciler.isProviderUpToDate(ap, secret, nil)).To(BeTrue())
		})

		It("should return false when client SecretResourceVersion does not match", func() {
			ap := &renovatev1beta1.AuthProvider{
				Name:       "test-provider-ver-mismatch",
				Generation: 1,
				Status: renovatev1beta1.AuthProviderStatus{
					Registered:            true,
					SecretResourceVersion: "v1",
				},
			}
			ap.SetCondition(renovatev1beta1.ConditionReady, metav1.ConditionTrue, "Ready", "")

			authManager.Register(&mockAuthProvider{name: ap.Name})

			secret := &corev1.Secret{}
			secret.ResourceVersion = "v2"

			Expect(reconciler.isProviderUpToDate(ap, secret, nil)).To(BeFalse())
		})

		It("should return false when condition Ready is not true", func() {
			ap := &renovatev1beta1.AuthProvider{
				Name:       "test-provider-not-ready",
				Generation: 1,
				Status: renovatev1beta1.AuthProviderStatus{
					Registered:            true,
					SecretResourceVersion: "v1",
				},
			}

			secret := &corev1.Secret{}
			secret.ResourceVersion = "v1"

			Expect(reconciler.isProviderUpToDate(ap, secret, nil)).To(BeFalse())
		})
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

func (m *mockAuthProvider) DisplayName() string {
	return m.name
}

func (m *mockAuthProvider) IconURL() string {
	return ""
}

func (m *mockAuthProvider) LoginURL(_, _ string) string {
	return ""
}

func (m *mockAuthProvider) HandleCallback(_ context.Context, _, _ string) (*auth.AuthenticatedUser, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAuthProvider) RefreshToken(_ context.Context, _ string) (*auth.AuthenticatedUser, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAuthProvider) ValidateToken(_ context.Context, _ string) (*auth.AuthenticatedUser, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAuthProvider) GetUserRepos(_ context.Context, _ *http.Client) (map[string]bool, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAuthProvider) IsUserRepo(_ context.Context, _ *http.Client, _ string) (bool, error) {
	return false, errors.New("not implemented")
}
