package auth

import (
	"context"
	"errors"
	"net/http"
)

var errNotImplemented = errors.New("not implemented")

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
	ForgeURL() string
	LoginURL(state string) string
	HandleCallback(ctx context.Context, code string) (*AuthenticatedUser, error)
	GetAccessChecker(token string) (RepoAccessChecker, error)
}

type RepoAccessChecker interface {
	GetAccessibleRepos(ctx context.Context) (map[string]bool, error)
}

type Manager struct {
	providers map[string]AuthProvider
}

func NewManager() *Manager {
	return &Manager{
		providers: make(map[string]AuthProvider),
	}
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

func GetProviderFromSession(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}

	session, err := decryptSession(cookie.Value)
	if err != nil {
		return "", false
	}

	return session.Provider, session.Provider != ""
}
