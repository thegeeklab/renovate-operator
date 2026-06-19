package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

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
		manager = NewManager(false)
		manager.Register(&testAuthProvider{name: "gitea-prod", provType: ProviderTypeGitea})

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
			manager = NewManager(false)
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

			It("should not treat arbitrary paths under /auth/ as public", func() {
				req := httptest.NewRequest(http.MethodGet, "/auth/some-arbitrary-path", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusFound))
				Expect(rec.Header().Get("Location")).To(Equal("/login"))
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

			It("should redirect /events to /login without session", func() {
				req := httptest.NewRequest(http.MethodGet, "/events", nil)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusFound))
				Expect(rec.Header().Get("Location")).To(Equal("/login"))
			})
		})

		Describe("protected paths with valid session", func() {
			var sessionCookie *http.Cookie

			BeforeEach(func() {
				session := SessionData{
					Email:       "test@example.com",
					Name:        "Test User",
					AccessToken: "token",
					Provider:    "gitea-prod",
				}

				req := httptest.NewRequest(http.MethodGet, "/setup-session", nil)
				setupRec := httptest.NewRecorder()

				setupHandler := manager.Session.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					SetSessionData(r.Context(), manager.Session, session)
					w.WriteHeader(http.StatusOK)
				}))
				setupHandler.ServeHTTP(setupRec, req)

				cookies := setupRec.Result().Cookies()
				for _, c := range cookies {
					if c.Name == sessionCookieName {
						sessionCookie = c

						break
					}
				}
			})

			It("should allow access to UI paths", func() {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(sessionCookie)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should allow access to API paths", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/gitrepos", nil)
				req.AddCookie(sessionCookie)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should put session data accessible via GetSessionData", func() {
				var sessionInCtx SessionData

				var ok bool

				ctxHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					sessionInCtx, ok = GetSessionData(r.Context(), manager.Session)

					w.WriteHeader(http.StatusOK)
				})

				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(sessionCookie)
				Middleware(manager)(ctxHandler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(ok).To(BeTrue())
				Expect(sessionInCtx.Email).To(Equal("test@example.com"))
				Expect(sessionInCtx.Name).To(Equal("Test User"))
				Expect(sessionInCtx.Provider).To(Equal("gitea-prod"))
			})
		})

		Describe("protected paths with invalid provider", func() {
			var sessionCookie *http.Cookie

			BeforeEach(func() {
				session := SessionData{
					Email:    "test@example.com",
					Provider: "unknown-provider",
				}

				req := httptest.NewRequest(http.MethodGet, "/setup-session", nil)
				setupRec := httptest.NewRecorder()

				setupHandler := manager.Session.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					SetSessionData(r.Context(), manager.Session, session)
					w.WriteHeader(http.StatusOK)
				}))
				setupHandler.ServeHTTP(setupRec, req)

				cookies := setupRec.Result().Cookies()
				for _, c := range cookies {
					if c.Name == sessionCookieName {
						sessionCookie = c

						break
					}
				}
			})

			It("should redirect UI paths to /login", func() {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(sessionCookie)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusFound))
				Expect(rec.Header().Get("Location")).To(Equal("/login"))
			})

			It("should return 401 for API paths", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/gitrepos", nil)
				req.AddCookie(sessionCookie)
				Middleware(manager)(handler).ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
