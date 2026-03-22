package gitrepo

import (
	"context"
	"fmt"

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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("GitRepo Component - Webhook Logic", func() {
	var (
		ctx        context.Context
		scheme     *runtime.Scheme
		fakeClient client.Client
		instance   *renovatev1beta1.GitRepo
		renovate   *renovatev1beta1.RenovateConfig
		reconciler *Reconciler
		mockMgr    *mocks.WebhookManager
		secretName string
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
			},
			Spec: renovatev1beta1.RenovateConfigSpec{
				Platform: renovatev1beta1.PlatformSpec{
					Type: "gitea",
				},
			},
		}

		secretName = fmt.Sprintf("%s-webhook-secret", instance.Name)

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(instance).
			Build()

		var err error

		reconciler, err = NewReconciler(fakeClient, scheme, instance, renovate)
		Expect(err).NotTo(HaveOccurred())

		mockMgr = mocks.NewWebhookManager(GinkgoT())
		reconciler.ProviderFactory = func(
			context.Context, client.Client, *renovatev1beta1.GitRepo, *renovatev1beta1.RenovateConfig,
		) (provider.WebhookManager, error) {
			return mockMgr, nil
		}
	})

	AfterEach(func() {
		mockMgr.AssertExpectations(GinkgoT())
	})

	Describe("createWebhook", func() {
		Context("when the secret exists", func() {
			BeforeEach(func() {
				webhookSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: instance.Namespace,
					},
					Data: map[string][]byte{
						"secret": []byte("test-secret-value"),
					},
				}
				Expect(fakeClient.Create(ctx, webhookSecret)).To(Succeed())
			})

			It("should successfully ensure webhook and update the instance WebhookID", func() {
				mockMgr.On("EnsureWebhook", mock.Anything, "org/repo", DummyWebhookURL, "test-secret-value").
					Return("mock-id-123", nil).
					Once()

				_, err := reconciler.createWebhook(ctx)
				Expect(err).NotTo(HaveOccurred())

				updated := &renovatev1beta1.GitRepo{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updated)).To(Succeed())
				Expect(updated.Spec.WebhookID).To(Equal("mock-id-123"))
			})

			It("should not update the instance if the WebhookID is already correct", func() {
				instance.Spec.WebhookID = "mock-id-123"
				Expect(fakeClient.Update(ctx, instance)).To(Succeed())

				mockMgr.On("EnsureWebhook", mock.Anything, "org/repo", DummyWebhookURL, "test-secret-value").
					Return("mock-id-123", nil).
					Once()

				before := &renovatev1beta1.GitRepo{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), before)).To(Succeed())

				_, err := reconciler.createWebhook(ctx)
				Expect(err).NotTo(HaveOccurred())

				after := &renovatev1beta1.GitRepo{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), after)).To(Succeed())
				Expect(after.ResourceVersion).To(Equal(before.ResourceVersion))
			})

			It("should update the instance if the WebhookID changed on the provider (manual deletion recovery)", func() {
				instance.Spec.WebhookID = "old-id-999"
				Expect(fakeClient.Update(ctx, instance)).To(Succeed())

				mockMgr.On("EnsureWebhook", mock.Anything, "org/repo", DummyWebhookURL, "test-secret-value").
					Return("new-id-124", nil).
					Once()

				_, err := reconciler.createWebhook(ctx)
				Expect(err).NotTo(HaveOccurred())

				updated := &renovatev1beta1.GitRepo{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updated)).To(Succeed())
				Expect(updated.Spec.WebhookID).To(Equal("new-id-124"))
			})
		})

		It("should fail if the webhook secret is missing", func() {
			_, err := reconciler.createWebhook(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get webhook secret"))
		})

		It("should return safely without error if the provider is not implemented", func() {
			reconciler.ProviderFactory = func(
				context.Context, client.Client, *renovatev1beta1.GitRepo, *renovatev1beta1.RenovateConfig,
			) (provider.WebhookManager, error) {
				return nil, provider.ErrNotImplemented
			}

			_, err := reconciler.createWebhook(ctx)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("deleteWebhook", func() {
		It("should successfully delete webhook and clear the WebhookID", func() {
			controllerutil.AddFinalizer(instance, renovatev1beta1.FinalizerGitRepoWebhook)
			instance.Spec.WebhookID = "mock-id-123"
			Expect(fakeClient.Update(ctx, instance)).To(Succeed())

			Expect(fakeClient.Delete(ctx, instance)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), reconciler.instance)).To(Succeed())

			mockMgr.On("DeleteWebhook", mock.Anything, "org/repo", "mock-id-123").
				Return(nil).
				Once()

			_, err := reconciler.deleteWebhook(ctx)
			Expect(err).NotTo(HaveOccurred())

			updated := &renovatev1beta1.GitRepo{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updated)).To(Succeed())
			Expect(updated.Spec.WebhookID).To(BeEmpty())
		})

		It("should return early if the WebhookID is empty", func() {
			instance.Spec.WebhookID = ""
			Expect(fakeClient.Update(ctx, instance)).To(Succeed())

			_, err := reconciler.deleteWebhook(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should clear the WebhookID gracefully if the provider is not implemented", func() {
			controllerutil.AddFinalizer(instance, renovatev1beta1.FinalizerGitRepoWebhook)
			instance.Spec.WebhookID = "123"
			Expect(fakeClient.Update(ctx, instance)).To(Succeed())

			Expect(fakeClient.Delete(ctx, instance)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), reconciler.instance)).To(Succeed())

			reconciler.ProviderFactory = func(
				context.Context, client.Client, *renovatev1beta1.GitRepo, *renovatev1beta1.RenovateConfig,
			) (provider.WebhookManager, error) {
				return nil, provider.ErrNotImplemented
			}

			_, err := reconciler.deleteWebhook(ctx)
			Expect(err).NotTo(HaveOccurred())

			updated := &renovatev1beta1.GitRepo{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updated)).To(Succeed())
			Expect(updated.Spec.WebhookID).To(BeEmpty())
		})
	})
})
