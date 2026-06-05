package auth

import (
	"net/http"
	"net/http/httptest"

	"github.com/alexedwards/scs/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Session", func() {
	var sessionManager *scs.SessionManager

	BeforeEach(func() {
		var err error

		sessionManager, err = NewSessionManager("test-secret", false)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("NewSessionManager", func() {
		It("should create a session manager with correct cookie settings", func() {
			Expect(sessionManager).NotTo(BeNil())
			Expect(sessionManager.Cookie.Name).To(Equal(sessionCookieName))
			Expect(sessionManager.Cookie.HttpOnly).To(BeTrue())
			Expect(sessionManager.Cookie.Path).To(Equal("/"))
		})
	})

	Describe("SetSessionData/GetSessionData", func() {
		It("should store and retrieve session data", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			handler := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				original := SessionData{
					Email:       "test@example.com",
					Name:        "Test User",
					Subject:     "sub-123",
					AccessToken: "access-token-123",
					Provider:    "gitea-prod",
				}

				SetSessionData(r.Context(), sessionManager, original)

				retrieved, ok := GetSessionData(r.Context(), sessionManager)
				Expect(ok).To(BeTrue())
				Expect(retrieved.Email).To(Equal(original.Email))
				Expect(retrieved.Name).To(Equal(original.Name))
				Expect(retrieved.Subject).To(Equal(original.Subject))
				Expect(retrieved.AccessToken).To(Equal(original.AccessToken))
				Expect(retrieved.Provider).To(Equal(original.Provider))

				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("should return false when session data is not set", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			handler := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, ok := GetSessionData(r.Context(), sessionManager)
				Expect(ok).To(BeFalse())

				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("IsAuthenticated", func() {
		It("should return false when not authenticated", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			handler := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(IsAuthenticated(r.Context(), sessionManager)).To(BeFalse())

				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("should return true when authenticated", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			handler := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				SetSessionData(r.Context(), sessionManager, SessionData{
					Provider: "gitea-prod",
				})

				Expect(IsAuthenticated(r.Context(), sessionManager)).To(BeTrue())

				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GenerateCSRFToken/ValidateCSRFToken", func() {
		It("should generate and validate CSRF token", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			handler := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				token, err := GenerateCSRFToken(r.Context(), sessionManager)
				Expect(err).NotTo(HaveOccurred())
				Expect(token).NotTo(BeEmpty())

				Expect(ValidateCSRFToken(r.Context(), sessionManager, token)).To(BeTrue())
				Expect(ValidateCSRFToken(r.Context(), sessionManager, "wrong-token")).To(BeFalse())
				Expect(ValidateCSRFToken(r.Context(), sessionManager, "")).To(BeFalse())

				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("should return false when no CSRF token exists", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			handler := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(ValidateCSRFToken(r.Context(), sessionManager, "any-token")).To(BeFalse())

				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("DestroySession", func() {
		It("should remove session data", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			handler := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				SetSessionData(r.Context(), sessionManager, SessionData{
					Provider: "gitea-prod",
				})

				Expect(IsAuthenticated(r.Context(), sessionManager)).To(BeTrue())

				err := DestroySession(r.Context(), sessionManager)
				Expect(err).NotTo(HaveOccurred())

				Expect(IsAuthenticated(r.Context(), sessionManager)).To(BeFalse())

				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})
})
