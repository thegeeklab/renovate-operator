package auth

import (
	"context"
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type testAuthProvider struct {
	name     string
	provType string
	loginURL string
}

func (p *testAuthProvider) Type() string {
	return p.provType
}

func (p *testAuthProvider) Name() string {
	return p.name
}

func (p *testAuthProvider) DisplayName() string {
	return p.name
}

func (p *testAuthProvider) IconURL() string {
	return ""
}

func (p *testAuthProvider) LoginURL(state string) string {
	return p.loginURL + "?state=" + state
}

func (p *testAuthProvider) HandleCallback(ctx context.Context, code string) (*AuthenticatedUser, error) {
	return &AuthenticatedUser{
		Email:        "test@example.com",
		Name:         "Test User",
		Subject:      "sub-123",
		AccessToken:  "test-token",
		RefreshToken: "test-refresh-token",
		Provider:     p.name,
	}, nil
}

func (p *testAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (*AuthenticatedUser, error) {
	return &AuthenticatedUser{
		Email:        "test@example.com",
		Name:         "Test User",
		Subject:      "sub-123",
		AccessToken:  "refreshed-token",
		RefreshToken: "refreshed-refresh-token",
		Provider:     p.name,
	}, nil
}

func (p *testAuthProvider) GetUserRepos(
	ctx context.Context,
	client *http.Client,
) (map[string]bool, error) {
	return nil, errors.New("not implemented")
}

func (p *testAuthProvider) IsUserRepo(
	ctx context.Context,
	client *http.Client,
	fullName string,
) (bool, error) {
	return false, errors.New("not implemented")
}

var _ = Describe("Manager", func() {
	var manager *Manager

	BeforeEach(func() {
		manager = NewManager(false)
	})

	Describe("NewManager", func() {
		It("should create a new manager", func() {
			Expect(manager).NotTo(BeNil())
		})

		It("should start with no providers", func() {
			Expect(manager.IsEnabled()).To(BeFalse())
			Expect(manager.List()).To(BeEmpty())
		})
	})

	Describe("Register", func() {
		It("should register a provider", func() {
			provider := &testAuthProvider{name: "gitea-prod", provType: ProviderTypeGitea}
			manager.Register(provider)

			Expect(manager.IsEnabled()).To(BeTrue())
			Expect(manager.List()).To(HaveLen(1))
		})

		It("should register multiple providers", func() {
			manager.Register(&testAuthProvider{name: "gitea-prod", provType: ProviderTypeGitea})
			manager.Register(&testAuthProvider{name: "gitea-staging", provType: ProviderTypeGitea})

			Expect(manager.IsEnabled()).To(BeTrue())
			Expect(manager.List()).To(HaveLen(2))
		})
	})

	Describe("Get", func() {
		BeforeEach(func() {
			manager.Register(&testAuthProvider{name: "gitea-prod", provType: ProviderTypeGitea})
		})

		It("should return a registered provider", func() {
			provider, ok := manager.Get("gitea-prod")
			Expect(ok).To(BeTrue())
			Expect(provider.Name()).To(Equal("gitea-prod"))
			Expect(provider.Type()).To(Equal(ProviderTypeGitea))
		})

		It("should return false for unregistered provider", func() {
			provider, ok := manager.Get("unknown")
			Expect(ok).To(BeFalse())
			Expect(provider).To(BeNil())
		})
	})

	Describe("List", func() {
		BeforeEach(func() {
			manager.Register(&testAuthProvider{name: "gitea-prod", provType: ProviderTypeGitea})
			manager.Register(&testAuthProvider{name: "gitea-staging", provType: ProviderTypeGitea})
		})

		It("should return all registered providers", func() {
			providers := manager.List()
			Expect(providers).To(HaveLen(2))
		})
	})

	Describe("IsEnabled", func() {
		It("should return false when no providers registered", func() {
			Expect(manager.IsEnabled()).To(BeFalse())
		})

		It("should return true when providers registered", func() {
			manager.Register(&testAuthProvider{name: "gitea-prod", provType: ProviderTypeGitea})
			Expect(manager.IsEnabled()).To(BeTrue())
		})
	})

	Describe("RefreshSessionToken", func() {
		BeforeEach(func() {
			manager.Register(&testAuthProvider{name: "gitea-prod", provType: ProviderTypeGitea})
		})

		It("should refresh token successfully", func() {
			session := &SessionData{
				Email:        "test@example.com",
				AccessToken:  "old-token",
				RefreshToken: "old-refresh-token",
				Provider:     "gitea-prod",
			}

			refreshed, err := manager.RefreshSessionToken(context.Background(), session)
			Expect(err).NotTo(HaveOccurred())
			Expect(refreshed.AccessToken).To(Equal("refreshed-token"))
			Expect(refreshed.RefreshToken).To(Equal("refreshed-refresh-token"))
			Expect(refreshed.Provider).To(Equal("gitea-prod"))
		})

		It("should return error when provider not found", func() {
			session := &SessionData{
				Email:        "test@example.com",
				AccessToken:  "old-token",
				RefreshToken: "old-refresh-token",
				Provider:     "unknown-provider",
			}

			refreshed, err := manager.RefreshSessionToken(context.Background(), session)
			Expect(err).To(Equal(ErrInvalidProvider))
			Expect(refreshed).To(BeNil())
		})

		It("should return error when no refresh token", func() {
			session := &SessionData{
				Email:       "test@example.com",
				AccessToken: "old-token",
				Provider:    "gitea-prod",
			}

			refreshed, err := manager.RefreshSessionToken(context.Background(), session)
			Expect(err).To(Equal(ErrNoRefreshToken))
			Expect(refreshed).To(BeNil())
		})

		It("should return error when provider refresh fails", func() {
			failingProvider := &failingAuthProvider{name: "gitea-fail"}
			manager.Register(failingProvider)

			session := &SessionData{
				Email:        "test@example.com",
				AccessToken:  "old-token",
				RefreshToken: "bad-refresh-token",
				Provider:     "gitea-fail",
			}

			refreshed, err := manager.RefreshSessionToken(context.Background(), session)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("refresh failed"))
			Expect(refreshed).To(BeNil())
		})
	})
})
