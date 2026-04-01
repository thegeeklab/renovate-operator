package gitrepo

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("GitRepo Component - Webhook Secret Logic", func() {
	var (
		ctx        context.Context
		scheme     *runtime.Scheme
		fakeClient client.Client
		instance   *renovatev1beta1.GitRepo
		reconciler *Reconciler
		secretKey  client.ObjectKey
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())

		instance = &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-repo",
				Namespace: "default",
				UID:       "test-uid-123",
			},
		}

		secretKey = client.ObjectKey{
			Name:      fmt.Sprintf("%s-webhook-secret", instance.Name),
			Namespace: instance.Namespace,
		}

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(instance).
			Build()

		var err error

		externalURL := "https://renovate.example.com"
		reconciler, err = NewReconciler(fakeClient, scheme, externalURL, instance, &renovatev1beta1.RenovateConfig{})
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("reconcileWebhookSecret", func() {
		It("should create a new secret with a secure token if it does not exist", func() {
			_, err := reconciler.reconcileWebhookSecret(ctx)
			Expect(err).NotTo(HaveOccurred())

			secret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, secretKey, secret)).To(Succeed())

			// 32 byte random -> 64 char hex string
			Expect(secret.Data).To(HaveKey("secret"))
			Expect(string(secret.Data["secret"])).To(HaveLen(64))

			Expect(secret.OwnerReferences).To(HaveLen(1))
			Expect(secret.OwnerReferences[0].Name).To(Equal(instance.Name))
			Expect(secret.OwnerReferences[0].UID).To(Equal(instance.UID))
		})

		It("should not overwrite an existing secret", func() {
			existingToken := "existing-token-value"
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretKey.Name,
					Namespace: secretKey.Namespace,
				},
				Data: map[string][]byte{
					"secret": []byte(existingToken),
				},
			}
			Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

			_, err := reconciler.reconcileWebhookSecret(ctx)
			Expect(err).NotTo(HaveOccurred())

			secret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, secretKey, secret)).To(Succeed())
			Expect(string(secret.Data["secret"])).To(Equal(existingToken))
		})

		It("should return early if the resource is being deleted", func() {
			now := metav1.Now()
			reconciler.instance.SetDeletionTimestamp(&now)

			_, err := reconciler.reconcileWebhookSecret(ctx)
			Expect(err).NotTo(HaveOccurred())

			secret := &corev1.Secret{}
			err = fakeClient.Get(ctx, secretKey, secret)
			Expect(client.IgnoreNotFound(err)).To(Succeed())
		})
	})

	Describe("generateSecureToken", func() {
		It("should generate a valid 64-character hex string", func() {
			token, err := generateSecureToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).To(MatchRegexp(`^[0-9a-f]{64}$`))
		})

		It("should generate unique tokens on consecutive calls", func() {
			token1, _ := generateSecureToken()
			token2, _ := generateSecureToken()
			Expect(token1).NotTo(Equal(token2))
		})
	})
})
