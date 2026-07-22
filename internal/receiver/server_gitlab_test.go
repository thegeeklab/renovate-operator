package receiver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/provider"
	"github.com/thegeeklab/renovate-operator/internal/provider/factory"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("GitLab receiver server", func() {
	var (
		ctx       context.Context
		k8sClient client.Client
		server    *Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(batchv1.AddToScheme(scheme)).To(Succeed())

		repo := &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "project",
				Namespace: "default",
				Labels:    map[string]string{renovatev1beta1.LabelRenovator: "renovator-id"},
			},
		}
		config := &renovatev1beta1.RenovateConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "renovator",
				Namespace: "default",
				Labels:    map[string]string{renovatev1beta1.LabelRenovator: "renovator-id"},
			},
			Spec: renovatev1beta1.RenovateConfigSpec{Platform: renovatev1beta1.PlatformSpec{
				Type: renovatev1beta1.PlatformType_GITLAB,
				Token: corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "platform-token"},
					Key:                  "token",
				}},
			}},
		}
		webhookSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "project-webhook-secret", Namespace: "default"},
			Data:       map[string][]byte{renovatev1beta1.WebhookSecretDataKey: []byte("webhook-secret")},
		}
		platformSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "platform-token", Namespace: "default"},
			Data:       map[string][]byte{"token": []byte("platform-secret")},
		}

		k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			repo,
			config,
			webhookSecret,
			platformSecret,
		).Build()
		server = NewServer(DefaultServerConfig(), k8sClient, func(platformType renovatev1beta1.PlatformType) Receiver {
			if platformType == renovatev1beta1.PlatformType_GITLAB {
				return &gitlabTestReceiver{}
			}

			return nil
		})
	})

	It("patches the Renovate trigger annotation for a valid default-branch push", func() {
		req := gitlabServerRequest(`{"ref":"refs/heads/main","project":{"default_branch":"main"}}`)
		response := httptest.NewRecorder()

		server.router.ServeHTTP(response, req)
		Expect(response.Code).To(Equal(http.StatusAccepted))

		repo := &renovatev1beta1.GitRepo{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "project"}, repo)).To(Succeed())
		Expect(repo.Annotations).To(HaveKeyWithValue(
			renovatev1beta1.RenovatorOperation,
			renovatev1beta1.OperationRenovate,
		))
	})

	It("accepts an actor mismatch without triggering", func() {
		server.providerFactory = func(ctx context.Context, config factory.PlatformConfig) (provider.ProviderManager, error) {
			return &identityProvider{identity: "expected-bot"}, nil
		}
		req := gitlabServerRequest(`{
			"object_attributes":{"action":"update","description":"## Detected Dependencies\n- [x] update"},
			"user":{"username":"different-user"}
		}`)
		req.Header.Set("X-Gitlab-Event", "Merge Request Hook")

		response := httptest.NewRecorder()

		server.router.ServeHTTP(response, req)
		Expect(response.Code).To(Equal(http.StatusAccepted))

		repo := &renovatev1beta1.GitRepo{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "project"}, repo)).To(Succeed())
		Expect(repo.Annotations).NotTo(HaveKey(renovatev1beta1.RenovatorOperation))
	})
})

func gitlabServerRequest(body string) *http.Request {
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/hooks/default/project",
		strings.NewReader(body),
	)
	req.Header.Set("X-Gitlab-Token", "webhook-secret")
	req.Header.Set("X-Gitlab-Event", "Push Hook")

	return req
}

type gitlabTestReceiver struct{}

var _ Receiver = (*gitlabTestReceiver)(nil)

func (p *gitlabTestReceiver) Validate(req *http.Request, secretToken, body []byte) error {
	if req.Header.Get("X-Gitlab-Token") != string(secretToken) {
		return errors.New("invalid token")
	}

	return nil
}

func (p *gitlabTestReceiver) Parse(req *http.Request, body []byte) (ParseResult, error) {
	if req.Header.Get("X-Gitlab-Event") == "Push Hook" {
		return ParseResult{ShouldTrigger: true}, nil
	}

	return ParseResult{
		ShouldTrigger:    true,
		RequireUserCheck: true,
		User:             "different-user",
	}, nil
}

type identityProvider struct {
	identity string
}

var _ provider.ProviderManager = (*identityProvider)(nil)

func (p *identityProvider) GetIdentity() (string, error) {
	return p.identity, nil
}

func (p *identityProvider) EnsureWebhook(
	ctx context.Context,
	repoName, webhookURL, secret string,
) (string, error) {
	return "", nil
}

func (p *identityProvider) DeleteWebhook(ctx context.Context, repoName, webhookID string) error {
	return nil
}

func (p *identityProvider) RepoURL(ctx context.Context, repoName string) (string, error) {
	return "", nil
}

func (p *identityProvider) ListRepos(
	ctx context.Context,
	opts provider.ListReposOptions,
) ([]provider.Repo, error) {
	return nil, nil
}
