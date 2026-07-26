package receiver_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	testifymock "github.com/stretchr/testify/mock"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/receiver"
	"github.com/thegeeklab/renovate-operator/internal/receiver/mocks"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNamespace   = "default"
	testGitRepoName = "project"
	testRenovatorID = "renovator-id"
)

var _ = Describe("Server", func() {
	var (
		ctx        context.Context
		k8sClient  client.Client
		mockRecv   *mocks.Receiver
		server     *receiver.Server
		baseConfig renovatev1beta1.RenovateConfig
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(batchv1.AddToScheme(scheme)).To(Succeed())

		repo := &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testGitRepoName,
				Namespace: testNamespace,
				Labels:    map[string]string{renovatev1beta1.LabelRenovator: testRenovatorID},
			},
		}
		baseConfig = renovatev1beta1.RenovateConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testRenovatorID,
				Namespace: testNamespace,
				Labels:    map[string]string{renovatev1beta1.LabelRenovator: testRenovatorID},
			},
			Spec: renovatev1beta1.RenovateConfigSpec{Platform: renovatev1beta1.PlatformSpec{
				Type: renovatev1beta1.PlatformType_GITHUB,
				Token: corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "platform-token"},
					Key:                  "token",
				}},
			}},
		}
		webhookSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: testGitRepoName + "-webhook-secret", Namespace: testNamespace},
			Data:       map[string][]byte{renovatev1beta1.WebhookSecretDataKey: []byte("webhook-secret")},
		}
		platformSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "platform-token", Namespace: testNamespace},
			Data:       map[string][]byte{"token": []byte("platform-secret")},
		}

		k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			repo,
			&baseConfig,
			webhookSecret,
			platformSecret,
		).Build()

		mockRecv = mocks.NewReceiver(GinkgoT())
	})

	JustBeforeEach(func() {
		server = receiver.NewServer(receiver.DefaultServerConfig(), k8sClient, func(
			platformType renovatev1beta1.PlatformType,
		) receiver.Receiver {
			return mockRecv
		}, nil)
	})

	DescribeTable(
		"pushes patch the Renovate trigger annotation for every platform",
		func(platformType string) {
			config := baseConfig.DeepCopy()
			config.Spec.Platform.Type = renovatev1beta1.PlatformType(platformType)
			Expect(k8sClient.Update(ctx, config)).To(Succeed())

			mockRecv.On("Validate", testifymock.Anything, testifymock.Anything, testifymock.Anything).Return(nil)
			mockRecv.On("Parse", testifymock.Anything, testifymock.Anything).
				Return(receiver.ParseResult{ShouldTrigger: true}, nil)

			req := httptest.NewRequest(http.MethodPost, "/hooks/default/project", strings.NewReader("{}"))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, req)
			Expect(response.Code).To(Equal(http.StatusAccepted))

			repo := &renovatev1beta1.GitRepo{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace, Name: testGitRepoName,
			}, repo)).To(Succeed())
			Expect(repo.Annotations).To(HaveKeyWithValue(
				renovatev1beta1.RenovatorOperation,
				renovatev1beta1.OperationRenovate,
			))
		},
		Entry("GitHub", "github"),
		Entry("Gitea", "gitea"),
		Entry("GitLab", "gitlab"),
	)

	It("returns 403 on signature validation failure", func() {
		mockRecv.On("Validate", testifymock.Anything, testifymock.Anything, testifymock.Anything).
			Return(errors.New("invalid signature"))

		req := httptest.NewRequest(http.MethodPost, "/hooks/default/project", strings.NewReader("{}"))
		response := httptest.NewRecorder()

		server.ServeHTTP(response, req)
		Expect(response.Code).To(Equal(http.StatusForbidden))
	})

	It("returns 202 without patching when webhook does not trigger", func() {
		mockRecv.On("Validate", testifymock.Anything, testifymock.Anything, testifymock.Anything).Return(nil)
		mockRecv.On("Parse", testifymock.Anything, testifymock.Anything).
			Return(receiver.ParseResult{ShouldTrigger: false}, nil)

		req := httptest.NewRequest(http.MethodPost, "/hooks/default/project", strings.NewReader("{}"))
		response := httptest.NewRecorder()

		server.ServeHTTP(response, req)
		Expect(response.Code).To(Equal(http.StatusAccepted))

		repo := &renovatev1beta1.GitRepo{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{
			Namespace: testNamespace, Name: testGitRepoName,
		}, repo)).To(Succeed())
		Expect(repo.Annotations).NotTo(HaveKey(renovatev1beta1.RenovatorOperation))
	})
})
