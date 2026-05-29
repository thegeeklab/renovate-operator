package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Middleware", func() {
	var (
		manager *Manager
		handler http.Handler
		rec     *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		manager = NewManager()
		manager.Register(&testAuthProvider{name: "gitea-prod", provType: ProviderTypeGitea})
		InitSessionKey("test-secret")

		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})
	})

	JustBeforeEach(func() {
		rec = httptest.NewRecorder()
	})

	Describe("When auth is disabled", func() {
		BeforeEach(func() {
			manager = NewManager()
		})

		It("should pass through all requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			Middleware(manager)(handler).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(Equal("OK"))
		})
	})

	Describe("When auth is enabled", func() {
		Describe("public paths", func() {
			It("should pass through requests to /auth/login", func() {
				req := httptest.NewRequest(http.MethodGet, "/auth/login?provider=gitea-prod", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should pass through requests to /auth/callback", func() {
				req := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should pass through requests to /auth/logout", func() {
				req := httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should allow access to /health", func() {
				req := httptest.NewRequest(http.MethodGet, "/health", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should allow access to /healthz", func() {
				req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should allow access to /readyz", func() {
				req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should pass through arbitrary paths under /auth/", func() {
				req := httptest.NewRequest(http.MethodGet, "/auth/some-arbitrary-path", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(Equal("OK"))
			})

			It("should not treat /healthz/extra as public", func() {
				req := httptest.NewRequest(http.MethodGet, "/healthz/extra", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusFound))
				Expect(rec.Header().Get("Location")).To(Equal("/login"))
			})
		})

		Describe("protected paths without session", func() {
			It("should redirect UI paths to /login", func() {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusFound))
				Expect(rec.Header().Get("Location")).To(Equal("/login"))
			})

			It("should return 401 for API paths", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/gitrepos", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))

				var resp map[string]string

				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp["error"]).To(Equal("unauthorized"))
			})

			It("should return 401 for /events without session", func() {
				req := httptest.NewRequest(http.MethodGet, "/events", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
				Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))

				var resp map[string]string

				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp["error"]).To(Equal("unauthorized"))
			})
		})

		Describe("protected paths with valid session", func() {
			var validSession string

			BeforeEach(func() {
				session := SessionData{
					Email:       "test@example.com",
					Name:        "Test User",
					AccessToken: "token",
					Provider:    "gitea-prod",
					Expiry:      time.Now().Add(time.Hour),
				}

				var err error

				validSession, err = encryptSession(session)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow access to UI paths", func() {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(&http.Cookie{
					Name:  sessionCookieName,
					Value: validSession,
				})
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should allow access to API paths", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/gitrepos", nil)
				req.AddCookie(&http.Cookie{
					Name:  sessionCookieName,
					Value: validSession,
				})
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should put session in context", func() {
				var sessionInCtx SessionData

				var ok bool

				ctxHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					sessionInCtx, ok = SessionFromContext(r.Context())

					w.WriteHeader(http.StatusOK)
				})

				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(&http.Cookie{
					Name:  sessionCookieName,
					Value: validSession,
				})
				Middleware(manager)(ctxHandler).ServeHTTP(rec, req)

				Expect(ok).To(BeTrue())
				Expect(sessionInCtx.Email).To(Equal("test@example.com"))
				Expect(sessionInCtx.Name).To(Equal("Test User"))
				Expect(sessionInCtx.Provider).To(Equal("gitea-prod"))
			})
		})

		Describe("protected paths with expired session", func() {
			var expiredSession string

			BeforeEach(func() {
				session := SessionData{
					Email:    "test@example.com",
					Provider: "gitea-prod",
					Expiry:   time.Now().Add(-time.Hour),
				}

				var err error

				expiredSession, err = encryptSession(session)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should redirect UI paths to /login", func() {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(&http.Cookie{
					Name:  sessionCookieName,
					Value: expiredSession,
				})
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusFound))
				Expect(rec.Header().Get("Location")).To(Equal("/login"))
			})

			It("should return 401 for API paths", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/gitrepos", nil)
				req.AddCookie(&http.Cookie{
					Name:  sessionCookieName,
					Value: expiredSession,
				})
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should return 401 for /events with expired session", func() {
				req := httptest.NewRequest(http.MethodGet, "/events", nil)
				req.AddCookie(&http.Cookie{
					Name:  sessionCookieName,
					Value: expiredSession,
				})
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})
		})

		Describe("protected paths with invalid provider", func() {
			var validSession string

			BeforeEach(func() {
				session := SessionData{
					Email:    "test@example.com",
					Provider: "unknown-provider",
					Expiry:   time.Now().Add(time.Hour),
				}

				var err error

				validSession, err = encryptSession(session)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should redirect UI paths to /login", func() {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(&http.Cookie{
					Name:  sessionCookieName,
					Value: validSession,
				})
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusFound))
				Expect(rec.Header().Get("Location")).To(Equal("/login"))
			})

			It("should return 401 for API paths", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/gitrepos", nil)
				req.AddCookie(&http.Cookie{
					Name:  sessionCookieName,
					Value: validSession,
				})
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
