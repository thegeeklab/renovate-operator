package receiver_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/mock"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metrics"
	"github.com/thegeeklab/renovate-operator/internal/receiver"
	"github.com/thegeeklab/renovate-operator/internal/receiver/mocks"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
			Name:      testGitRepoName,
			Namespace: testNamespace,
			Labels:    map[string]string{renovatev1beta1.LabelRenovator: testRenovatorID},
		}
		baseConfig = renovatev1beta1.RenovateConfig{
			Name:      testRenovatorID,
			Namespace: testNamespace,
			Labels:    map[string]string{renovatev1beta1.LabelRenovator: testRenovatorID},
			Spec: renovatev1beta1.RenovateConfigSpec{Platform: renovatev1beta1.PlatformSpec{
				Type: renovatev1beta1.PlatformType_GITHUB,
				Token: corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					Name: "platform-token",
					Key:  "token",
				}},
			}},
		}
		webhookSecret := &corev1.Secret{
			Name: testGitRepoName + "-webhook-secret", Namespace: testNamespace,
			Data: map[string][]byte{renovatev1beta1.WebhookSecretDataKey: []byte("webhook-secret")},
		}
		platformSecret := &corev1.Secret{
			Name: "platform-token", Namespace: testNamespace,
			Data: map[string][]byte{"token": []byte("platform-secret")},
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

			mockRecv.On("Validate", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			mockRecv.On("Parse", mock.Anything, mock.Anything).
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
		mockRecv.On("Validate", mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("invalid signature"))

		req := httptest.NewRequest(http.MethodPost, "/hooks/default/project", strings.NewReader("{}"))
		response := httptest.NewRecorder()

		server.ServeHTTP(response, req)
		Expect(response.Code).To(Equal(http.StatusForbidden))
	})

	It("returns 202 without patching when webhook does not trigger", func() {
		mockRecv.On("Validate", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockRecv.On("Parse", mock.Anything, mock.Anything).
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

var _ = Describe("Server Metrics", func() {
	var (
		k8sClient client.Client
		mockRecv  *mocks.Receiver
		server    *receiver.Server
		reg       *prometheus.Registry
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(batchv1.AddToScheme(scheme)).To(Succeed())

		repo := &renovatev1beta1.GitRepo{
			Name:      testGitRepoName,
			Namespace: testNamespace,
			Labels:    map[string]string{renovatev1beta1.LabelRenovator: testRenovatorID},
		}
		config := &renovatev1beta1.RenovateConfig{
			Name:      testRenovatorID,
			Namespace: testNamespace,
			Labels:    map[string]string{renovatev1beta1.LabelRenovator: testRenovatorID},
			Spec: renovatev1beta1.RenovateConfigSpec{Platform: renovatev1beta1.PlatformSpec{
				Type: renovatev1beta1.PlatformType_GITHUB,
				Token: corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					Name: "platform-token",
					Key:  "token",
				}},
			}},
		}
		webhookSecret := &corev1.Secret{
			Name: testGitRepoName + "-webhook-secret", Namespace: testNamespace,
			Data: map[string][]byte{renovatev1beta1.WebhookSecretDataKey: []byte("webhook-secret")},
		}
		platformSecret := &corev1.Secret{
			Name: "platform-token", Namespace: testNamespace,
			Data: map[string][]byte{"token": []byte("platform-secret")},
		}

		k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			repo,
			config,
			webhookSecret,
			platformSecret,
		).Build()

		mockRecv = mocks.NewReceiver(GinkgoT())

		reg = prometheus.NewRegistry()

		server = receiver.NewServer(receiver.DefaultServerConfig(), k8sClient, func(
			platformType renovatev1beta1.PlatformType,
		) receiver.Receiver {
			return mockRecv
		}, metrics.New(reg, reg, 100))
	})

	metricCount := func(name string) int {
		families, err := reg.Gather()
		Expect(err).NotTo(HaveOccurred())

		for _, mf := range families {
			if mf.GetName() == name {
				return len(mf.GetMetric())
			}
		}

		return 0
	}

	It("records accepted webhook request", func() {
		mockRecv.On("Validate", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockRecv.On("Parse", mock.Anything, mock.Anything).
			Return(receiver.ParseResult{ShouldTrigger: true}, nil)

		req := httptest.NewRequest(http.MethodPost, "/hooks/default/project", strings.NewReader("{}"))
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusAccepted))

		Expect(metricCount("renovate_operator_webhook_requests_total")).To(Equal(1))
	})

	It("records signature verification failure", func() {
		mockRecv.On("Validate", mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("invalid signature"))

		req := httptest.NewRequest(http.MethodPost, "/hooks/default/project", strings.NewReader("{}"))
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusForbidden))

		Expect(metricCount("renovate_operator_webhook_signature_verification_failures_total")).To(Equal(1))
		Expect(metricCount("renovate_operator_webhook_requests_total")).To(Equal(1))
	})

	It("records ignored webhook request when no trigger needed", func() {
		mockRecv.On("Validate", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockRecv.On("Parse", mock.Anything, mock.Anything).
			Return(receiver.ParseResult{ShouldTrigger: false}, nil)

		req := httptest.NewRequest(http.MethodPost, "/hooks/default/project", strings.NewReader("{}"))
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusAccepted))

		expected := `
			# HELP renovate_operator_webhook_requests_total Total number of webhook requests by provider and result.
			# TYPE renovate_operator_webhook_requests_total counter
			renovate_operator_webhook_requests_total{provider="github",result="ignored"} 1
		`

		err := testutil.GatherAndCompare(reg, strings.NewReader(expected), "renovate_operator_webhook_requests_total")
		Expect(err).NotTo(HaveOccurred())
	})

	It("records secret resolution errors", func() {
		req := httptest.NewRequest(http.MethodPost, "/hooks/nonexistent/project", strings.NewReader("{}"))
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusNotFound))

		//nolint:lll
		expected := `
			# HELP renovate_operator_secret_resolution_errors_total Total number of Kubernetes Secret resolution errors by error type.
			# TYPE renovate_operator_secret_resolution_errors_total counter
			renovate_operator_secret_resolution_errors_total{error_type="not_found"} 1
		`

		err := testutil.GatherAndCompare(reg, strings.NewReader(expected), "renovate_operator_secret_resolution_errors_total")
		Expect(err).NotTo(HaveOccurred())
	})
})
