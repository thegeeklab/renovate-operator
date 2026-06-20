package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var ErrCallbackFailed = errors.New("callback failed")

var _ = Describe("Handlers", func() {
	var (
		manager *Manager
		rec     *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		manager = NewManager(false)
		manager.Register(&testAuthProvider{
			name:     "gitea-prod",
			provType: ProviderTypeGitea,
			loginURL: "https://gitea.example.com/login/oauth/authorize",
		})
	})

	JustBeforeEach(func() {
		rec = httptest.NewRecorder()
	})

	Describe("HandleLogin", func() {
		It("should redirect to provider login URL with state", func() {
			req := httptest.NewRequest(http.MethodGet, "/auth/login?provider=gitea-prod", nil)
			HandleLogin(manager, false).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusFound))

			location := rec.Header().Get("Location")
			Expect(location).To(ContainSubstring("https://gitea.example.com/login/oauth/authorize"))
			Expect(location).To(ContainSubstring("state="))

			cookies := rec.Result().Cookies()
			Expect(cookies).NotTo(BeEmpty())
		})

		It("should return 400 when provider is missing", func() {
			req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
			HandleLogin(manager, false).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 404 for unknown provider", func() {
			req := httptest.NewRequest(http.MethodGet, "/auth/login?provider=unknown", nil)
			HandleLogin(manager, false).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("should set Secure cookie when secureCookies is true", func() {
			req := httptest.NewRequest(http.MethodGet, "/auth/login?provider=gitea-prod", nil)
			HandleLogin(manager, true).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusFound))

			var stateCookie *http.Cookie

			for _, c := range rec.Result().Cookies() {
				if c.Name == stateCookieName {
					stateCookie = c

					break
				}
			}

			Expect(stateCookie).NotTo(BeNil())
			Expect(stateCookie.Secure).To(BeTrue())
		})
	})

	Describe("HandleLogout", func() {
		It("should clear session cookie and redirect to /", func() {
			req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)

			form := url.Values{}
			form.Set(csrfFormName, "any-token")
			req.PostForm = form

			handler := manager.SessionManager().LoadAndSave(HandleLogout(manager))
			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusFound))
			Expect(rec.Header().Get("Location")).To(Equal("/"))
		})

		It("should return 403 when CSRF token is missing", func() {
			req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)

			session := SessionData{
				Email:    "test@example.com",
				Name:     "Test User",
				Subject:  "sub-123",
				Provider: "gitea-prod",
			}

			handler := manager.SessionManager().LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				SetSessionData(r.Context(), manager.SessionManager(), session)

				HandleLogout(manager).ServeHTTP(w, r)
			}))
			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("should return 403 when CSRF token does not match", func() {
			req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)

			form := url.Values{}
			form.Set(csrfFormName, "wrong-token")
			req.PostForm = form

			session := SessionData{
				Email:    "test@example.com",
				Name:     "Test User",
				Subject:  "sub-123",
				Provider: "gitea-prod",
			}

			handler := manager.SessionManager().LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				SetSessionData(r.Context(), manager.SessionManager(), session)
				_, _ = GenerateCSRFToken(r.Context(), manager.SessionManager())

				HandleLogout(manager).ServeHTTP(w, r)
			}))
			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})
	})

	Describe("HandleAuthStatus", func() {
		It("should return enabled=false when auth is disabled", func() {
			disabledManager := NewManager(false)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
			rec := httptest.NewRecorder()
			HandleAuthStatus(disabledManager).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))

			var status map[string]any

			err := json.Unmarshal(rec.Body.Bytes(), &status)
			Expect(err).NotTo(HaveOccurred())
			Expect(status["enabled"]).To(BeFalse())
		})

		It("should return enabled=true, authenticated=false without session", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)

			handler := manager.SessionManager().LoadAndSave(HandleAuthStatus(manager))
			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))

			var status map[string]any

			err := json.Unmarshal(rec.Body.Bytes(), &status)
			Expect(err).NotTo(HaveOccurred())
			Expect(status["enabled"]).To(BeTrue())
			Expect(status["authenticated"]).To(BeFalse())
		})

		It("should return authenticated=true with valid session", func() {
			session := SessionData{
				Email:    "test@example.com",
				Name:     "Test User",
				Provider: "gitea-prod",
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)

			handler := manager.SessionManager().LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				SetSessionData(r.Context(), manager.SessionManager(), session)
				HandleAuthStatus(manager).ServeHTTP(w, r)
			}))
			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))

			var status map[string]any

			err := json.Unmarshal(rec.Body.Bytes(), &status)
			Expect(err).NotTo(HaveOccurred())
			Expect(status["enabled"]).To(BeTrue())
			Expect(status["authenticated"]).To(BeTrue())
			Expect(status["email"]).To(Equal("test@example.com"))
			Expect(status["name"]).To(Equal("Test User"))
			Expect(status["provider"]).To(Equal("gitea-prod"))
		})
	})

	Describe("HandleCallback", func() {
		It("should return 400 when state cookie is missing", func() {
			state, err := encodeState("gitea-prod")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&code=xyz", nil)
			HandleCallback(manager, false).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 400 when state does not match", func() {
			state, err := encodeState("gitea-prod")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&code=xyz", nil)
			req.AddCookie(&http.Cookie{
				Name:  stateCookieName,
				Value: "different-state",
			})
			HandleCallback(manager, false).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("should reject callback when state is malformed", func() {
			req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=not-encoded&code=xyz", nil)
			req.AddCookie(&http.Cookie{
				Name:  stateCookieName,
				Value: "not-encoded",
			})
			HandleCallback(manager, false).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 400 when code is missing", func() {
			state, err := encodeState("gitea-prod")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state, nil)
			req.AddCookie(&http.Cookie{
				Name:  stateCookieName,
				Value: state,
			})
			HandleCallback(manager, false).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 404 when state encodes an unknown provider", func() {
			state, err := encodeState("unknown-provider")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&code=xyz", nil)
			req.AddCookie(&http.Cookie{
				Name:  stateCookieName,
				Value: state,
			})
			HandleCallback(manager, false).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("should return 500 when provider callback fails", func() {
			failingProvider := &failingAuthProvider{name: "gitea-fail"}
			failingManager := NewManager(false)
			failingManager.Register(failingProvider)

			state, err := encodeState("gitea-fail")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&code=bad-code", nil)
			req.AddCookie(&http.Cookie{
				Name:  stateCookieName,
				Value: state,
			})
			HandleCallback(failingManager, false).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should derive provider from state, not from query", func() {
			spoofManager := NewManager(false)
			spoofManager.Register(&failingAuthProvider{name: "gitea-prod"})
			spoofManager.Register(&testAuthProvider{
				name:     "gitea-staging",
				provType: ProviderTypeGitea,
				loginURL: "https://staging.example.com/login",
			})

			state, err := encodeState("gitea-prod")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(
				http.MethodGet,
				"/auth/callback?state="+state+"&code=xyz&provider=gitea-staging",
				nil,
			)
			req.AddCookie(&http.Cookie{
				Name:  stateCookieName,
				Value: state,
			})
			HandleCallback(spoofManager, false).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should store refresh token and token expiry in session", func() {
			state, err := encodeState("gitea-prod")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&code=valid-code", nil)
			req.AddCookie(&http.Cookie{
				Name:  stateCookieName,
				Value: state,
			})

			var storedSession SessionData

			var ok bool

			handler := manager.SessionManager().LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				HandleCallback(manager, false).ServeHTTP(w, r)

				storedSession, ok = GetSessionData(r.Context(), manager.SessionManager())
			}))
			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusFound))
			Expect(ok).To(BeTrue())
			Expect(storedSession.RefreshToken).To(Equal("test-refresh-token"))
			Expect(storedSession.AccessToken).To(Equal("test-token"))
		})
	})
})

type failingAuthProvider struct {
	name string
}

func (p *failingAuthProvider) Type() string {
	return ProviderTypeGitea
}

func (p *failingAuthProvider) Name() string {
	return p.name
}

func (p *failingAuthProvider) LoginURL(state string) string {
	return "https://fail.example.com/login?state=" + url.QueryEscape(state)
}

func (p *failingAuthProvider) HandleCallback(ctx context.Context, code string) (*AuthenticatedUser, error) {
	return nil, ErrCallbackFailed
}

func (p *failingAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (*AuthenticatedUser, error) {
	return nil, errors.New("refresh failed")
}

func (p *failingAuthProvider) GetUserRepos(
	ctx context.Context,
	client *http.Client,
) (map[string]bool, error) {
	return nil, errors.New("not implemented")
}

func (p *failingAuthProvider) IsUserRepo(
	ctx context.Context,
	client *http.Client,
	fullName string,
) (bool, error) {
	return false, errors.New("not implemented")
}
