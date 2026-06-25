package gitrepo

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/provider"
	"github.com/thegeeklab/renovate-operator/internal/provider/mocks"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("GitRepo Component - Platform Info Logic", func() {
	var (
		ctx        context.Context
		scheme     *runtime.Scheme
		fakeClient client.Client
		instance   *renovatev1beta1.GitRepo
		renovate   *renovatev1beta1.RenovateConfig
		reconciler *Reconciler
		mockMgr    *mocks.ProviderManager
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		instance = &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-repo",
				Namespace: "default",
				UID:       "test-uid-123",
			},
			Spec: renovatev1beta1.GitRepoSpec{
				Name: "org/repo",
			},
		}

		renovate = &renovatev1beta1.RenovateConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-config",
				Namespace: "default",
				UID:       "test-renovate-uid-123",
			},
			Spec: renovatev1beta1.RenovateConfigSpec{
				Platform: renovatev1beta1.PlatformSpec{
					Type:     "gitea",
					Endpoint: "https://gitea.example.com/api/v1",
					Token: corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "token",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "token-secret",
							},
						},
					},
				},
			},
		}

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(instance).
			WithStatusSubresource(&renovatev1beta1.GitRepo{}).
			Build()

		tokenSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "token-secret",
				Namespace: instance.Namespace,
			},
			Data: map[string][]byte{
				"token": []byte("test-token"),
			},
		}
		Expect(fakeClient.Create(ctx, tokenSecret)).To(Succeed())

		var err error

		reconciler, err = NewReconciler(fakeClient, scheme, "https://renovate.example.com", nil, instance, renovate)
		Expect(err).NotTo(HaveOccurred())

		mockMgr = mocks.NewProviderManager(GinkgoT())
		reconciler.providerFactory = func(
			context.Context, provider.PlatformConfig,
		) (provider.ProviderManager, error) {
			return mockMgr, nil
		}
	})

	AfterEach(func() {
		mockMgr.AssertExpectations(GinkgoT())
	})

	Describe("reconcilePlatformInfo", func() {
		It("should successfully populate platform and repoURL in status", func() {
			mockMgr.On("RepoURL", mock.Anything, "org/repo").
				Return("https://gitea.example.com/org/repo", nil).
				Once()

			_, err := reconciler.reconcilePlatformInfo(ctx)
			Expect(err).NotTo(HaveOccurred())

			updated := &renovatev1beta1.GitRepo{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updated)).To(Succeed())
			Expect(updated.Status.Platform).To(Equal("gitea"))
			Expect(updated.Status.RepoURL).To(Equal("https://gitea.example.com/org/repo"))
		})

		It("should not update status if platform and repoURL are already correct", func() {
			instance.Status.Platform = "gitea"
			instance.Status.RepoURL = "https://gitea.example.com/org/repo"
			Expect(fakeClient.Status().Update(ctx, instance)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), reconciler.instance)).To(Succeed())

			mockMgr.On("RepoURL", mock.Anything, "org/repo").
				Return("https://gitea.example.com/org/repo", nil).
				Once()

			_, err := reconciler.reconcilePlatformInfo(ctx)
			Expect(err).NotTo(HaveOccurred())

			updated := &renovatev1beta1.GitRepo{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updated)).To(Succeed())
			Expect(updated.Status.Platform).To(Equal("gitea"))
			Expect(updated.Status.RepoURL).To(Equal("https://gitea.example.com/org/repo"))
		})

		It("should update status if platform changed", func() {
			instance.Status.Platform = "github"
			instance.Status.RepoURL = "https://github.com/org/repo"
			Expect(fakeClient.Status().Update(ctx, instance)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), reconciler.instance)).To(Succeed())

			mockMgr.On("RepoURL", mock.Anything, "org/repo").
				Return("https://gitea.example.com/org/repo", nil).
				Once()

			_, err := reconciler.reconcilePlatformInfo(ctx)
			Expect(err).NotTo(HaveOccurred())

			updated := &renovatev1beta1.GitRepo{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updated)).To(Succeed())
			Expect(updated.Status.Platform).To(Equal("gitea"))
			Expect(updated.Status.RepoURL).To(Equal("https://gitea.example.com/org/repo"))
		})

		It("should update status if repoURL changed", func() {
			instance.Status.Platform = "gitea"
			instance.Status.RepoURL = "https://old-gitea.example.com/org/repo"
			Expect(fakeClient.Status().Update(ctx, instance)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), reconciler.instance)).To(Succeed())

			mockMgr.On("RepoURL", mock.Anything, "org/repo").
				Return("https://gitea.example.com/org/repo", nil).
				Once()

			_, err := reconciler.reconcilePlatformInfo(ctx)
			Expect(err).NotTo(HaveOccurred())

			updated := &renovatev1beta1.GitRepo{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updated)).To(Succeed())
			Expect(updated.Status.Platform).To(Equal("gitea"))
			Expect(updated.Status.RepoURL).To(Equal("https://gitea.example.com/org/repo"))
		})

		It("should fail if the platform token secret is missing", func() {
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "token-secret", Namespace: instance.Namespace}}
			Expect(fakeClient.Delete(ctx, secret)).To(Succeed())

			_, err := reconciler.reconcilePlatformInfo(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get platform token secret"))
		})

		It("should return safely without error if the provider is not implemented", func() {
			reconciler.providerFactory = func(
				context.Context, provider.PlatformConfig,
			) (provider.ProviderManager, error) {
				return nil, provider.ErrNotImplemented
			}

			_, err := reconciler.reconcilePlatformInfo(ctx)
			Expect(err).NotTo(HaveOccurred())

			updated := &renovatev1beta1.GitRepo{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updated)).To(Succeed())
			Expect(updated.Status.Platform).To(BeEmpty())
			Expect(updated.Status.RepoURL).To(BeEmpty())
		})

		It("should fail if RepoURL returns an error", func() {
			mockMgr.On("RepoURL", mock.Anything, "org/repo").
				Return("", errors.New("failed to fetch repository")).
				Once()

			_, err := reconciler.reconcilePlatformInfo(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to fetch repository"))
		})
	})
})
