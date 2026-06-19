package auth

import (
	"context"
	"sync"

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
	mu        sync.RWMutex
	providers map[string]AuthProvider
	Session   *scs.SessionManager
	intended  bool
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
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[provider.Name()] = provider
}

func (m *Manager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.providers, name)
}

//nolint:ireturn
func (m *Manager) Get(name string) (AuthProvider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.providers[name]

	return p, ok
}

func (m *Manager) List() []AuthProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AuthProvider, 0, len(m.providers))
	for _, p := range m.providers {
		result = append(result, p)
	}

	return result
}

func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.providers) > 0
}

func (m *Manager) SetIntended(intended bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.intended = intended
}

func (m *Manager) IsIntended() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.intended
}
