package gitrepo

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("GitRepo Component - Finalizer Logic", func() {
	var (
		ctx        context.Context
		scheme     *runtime.Scheme
		fakeClient client.Client
		instance   *renovatev1beta1.GitRepo
		renovate   *renovatev1beta1.RenovateConfig
		reconciler *Reconciler
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

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(instance).
			Build()

		var err error

		reconciler, err = NewReconciler(fakeClient, scheme, instance, renovate)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("reconcileGitRepo", func() {
		It("should add the finalizer if it is not present", func() {
			_, err := reconciler.reconcileGitRepo(ctx)
			Expect(err).NotTo(HaveOccurred())

			updated := &renovatev1beta1.GitRepo{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updated)).To(Succeed())
			Expect(controllerutil.ContainsFinalizer(updated, renovatev1beta1.FinalizerGitRepoWebhook)).To(BeTrue())
		})

		It("should remove the finalizer if the resource is being deleted and the webhook is detached", func() {
			controllerutil.AddFinalizer(instance, renovatev1beta1.FinalizerGitRepoWebhook)
			instance.Spec.WebhookID = ""
			Expect(fakeClient.Update(ctx, instance)).To(Succeed())

			Expect(fakeClient.Delete(ctx, instance)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), reconciler.instance)).To(Succeed())

			_, err := reconciler.reconcileGitRepo(ctx)
			Expect(err).NotTo(HaveOccurred())

			updated := &renovatev1beta1.GitRepo{}
			err = fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updated)
			Expect(err).To(HaveOccurred())
			Expect(api_errors.IsNotFound(err)).To(BeTrue())
		})

		It("should NOT remove the finalizer if the resource is being deleted but the webhook is still attached", func() {
			controllerutil.AddFinalizer(instance, renovatev1beta1.FinalizerGitRepoWebhook)
			instance.Spec.WebhookID = "12345"
			Expect(fakeClient.Update(ctx, instance)).To(Succeed())

			Expect(fakeClient.Delete(ctx, instance)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), reconciler.instance)).To(Succeed())

			_, err := reconciler.reconcileGitRepo(ctx)
			Expect(err).NotTo(HaveOccurred())

			updated := &renovatev1beta1.GitRepo{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updated)).To(Succeed())
			Expect(controllerutil.ContainsFinalizer(updated, renovatev1beta1.FinalizerGitRepoWebhook)).To(BeTrue())
		})
	})
})
