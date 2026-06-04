package auth

import (
	"context"

	"github.com/alexedwards/scs/v2"
)

const (
	ProviderTypeGitea = "gitea"
)

type ProviderConfig struct {
	Name         string
	Type         string
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	ForgeURL     string
	AuthURL      string
	Insecure     bool
}

type AuthenticatedUser struct {
	Email       string
	Name        string
	Subject     string
	AccessToken string
	Provider    string
}

type AuthProvider interface {
	Type() string
	Name() string
	LoginURL(state string) string
	HandleCallback(ctx context.Context, code string) (*AuthenticatedUser, error)
	GetUserRepos(ctx context.Context, token string) (map[string]bool, error)
	IsUserRepo(ctx context.Context, token, fullName string) (bool, error)
}

type Manager struct {
	providers map[string]AuthProvider
	Session   *scs.SessionManager
}

func NewManager(sessionSecret string, secureCookies bool) (*Manager, error) {
	session, err := NewSessionManager(sessionSecret, secureCookies)
	if err != nil {
		return nil, err
	}

	return &Manager{
		providers: make(map[string]AuthProvider),
		Session:   session,
	}, nil
}

func (m *Manager) Register(provider AuthProvider) {
	m.providers[provider.Name()] = provider
}

//nolint:ireturn
func (m *Manager) Get(name string) (AuthProvider, bool) {
	p, ok := m.providers[name]

	return p, ok
}

func (m *Manager) List() []AuthProvider {
	result := make([]AuthProvider, 0, len(m.providers))
	for _, p := range m.providers {
		result = append(result, p)
	}

	return result
}

func (m *Manager) IsEnabled() bool {
	return len(m.providers) > 0
}
