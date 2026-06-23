package auth

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"
	"golang.org/x/sync/singleflight"
)

const (
	ProviderTypeGitea = "gitea"
)

var (
	ErrInvalidProvider = errors.New("invalid provider")
	ErrNoRefreshToken  = errors.New("no refresh token available")
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
	Email        string
	Name         string
	Subject      string
	AccessToken  string
	RefreshToken string
	TokenExpiry  time.Time
	Provider     string
}

type AuthProvider interface {
	Type() string
	Name() string
	LoginURL(state string) string
	HandleCallback(ctx context.Context, code string) (*AuthenticatedUser, error)
	RefreshToken(ctx context.Context, refreshToken string) (*AuthenticatedUser, error)
	GetUserRepos(ctx context.Context, client *http.Client) (map[string]bool, error)
	IsUserRepo(ctx context.Context, client *http.Client, fullName string) (bool, error)
}

type Manager struct {
	mu           sync.RWMutex
	providers    map[string]AuthProvider
	session      *scs.SessionManager
	intended     bool
	refreshGroup singleflight.Group
}

func NewManager(secureCookies bool) *Manager {
	session := NewSessionManager(secureCookies)

	return &Manager{
		providers: make(map[string]AuthProvider),
		session:   session,
	}
}

// SessionManager returns the session manager used by this auth manager.
func (m *Manager) SessionManager() *scs.SessionManager {
	return m.session
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

func (m *Manager) RefreshSessionToken(ctx context.Context, session *SessionData) (*SessionData, error) {
	m.mu.RLock()

	provider, ok := m.providers[session.Provider]

	m.mu.RUnlock()

	if !ok {
		return nil, ErrInvalidProvider
	}

	if session.RefreshToken == "" {
		return nil, ErrNoRefreshToken
	}

	user, err := provider.RefreshToken(ctx, session.RefreshToken)
	if err != nil {
		return nil, err
	}

	return &SessionData{
		Email:        user.Email,
		Name:         user.Name,
		Subject:      user.Subject,
		AccessToken:  user.AccessToken,
		RefreshToken: user.RefreshToken,
		TokenExpiry:  user.TokenExpiry,
		Provider:     session.Provider,
	}, nil
}
