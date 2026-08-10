package i18n

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var _ = Describe("Middleware", func() {
	var (
		bundle  *goi18n.Bundle
		handler http.Handler
	)

	BeforeEach(func() {
		bundle = NewBundle()
		handler = Middleware(bundle, MiddlewareConfig{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tr := FromContext(r.Context())
			Expect(tr).NotTo(BeNil(), "translator should be in context")

			w.WriteHeader(http.StatusOK)
		}))
	})

	It("sets Content-Language header", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Header().Get("Content-Language")).To(Equal("en"))
	})

	It("sets locale cookie on the response", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		cookies := rec.Result().Cookies()
		Expect(cookies).To(HaveLen(1))

		c := cookies[0]
		Expect(c.Name).To(Equal("locale"))
		Expect(c.Value).To(Equal("en"))
		Expect(c.Path).To(Equal("/"))
		Expect(c.SameSite).To(Equal(http.SameSiteLaxMode))
	})

	It("respects the locale cookie when set", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "locale", Value: "de"})

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Header().Get("Content-Language")).To(Equal("de"))
	})

	It("rejects an unknown locale cookie value", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "locale", Value: "fr"})

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Header().Get("Content-Language")).To(Equal("en"))
	})

	It("respects the Accept-Language header", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Language", "de-DE,de;q=0.9")

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Header().Get("Content-Language")).To(Equal("de"))
	})

	It("falls back to en when Accept-Language is unsupported", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Language", "fr-FR,fr;q=0.9")

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Header().Get("Content-Language")).To(Equal("en"))
	})

	It("cookie takes priority over Accept-Language", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "locale", Value: "de"})
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Header().Get("Content-Language")).To(Equal("de"))
	})

	It("injects translator into request context", func() {
		var capturedLocale string

		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tr := FromContext(r.Context())
			capturedLocale = tr.Locale()

			w.WriteHeader(http.StatusOK)
		})
		captureHandler := Middleware(bundle, MiddlewareConfig{})(inner)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "locale", Value: "de"})

		rec := httptest.NewRecorder()

		captureHandler.ServeHTTP(rec, req)

		Expect(capturedLocale).To(Equal("de"))
	})

	It("does not set locale cookie or Content-Language for API paths", func() {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(rec.Result().Cookies()).To(BeEmpty())
		Expect(rec.Header().Get("Content-Language")).To(BeEmpty())
	})

	It("still injects translator context for API paths", func() {
		var capturedLocale string

		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tr := FromContext(r.Context())
			capturedLocale = tr.Locale()

			w.WriteHeader(http.StatusOK)
		})
		apiHandler := Middleware(bundle, MiddlewareConfig{})(inner)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
		req.AddCookie(&http.Cookie{Name: "locale", Value: "de"})

		rec := httptest.NewRecorder()

		apiHandler.ServeHTTP(rec, req)

		Expect(capturedLocale).To(Equal("de"))
	})

	It("sets Secure flag on the cookie when configured", func() {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		secureHandler := Middleware(bundle, MiddlewareConfig{SecureCookies: true})(inner)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		secureHandler.ServeHTTP(rec, req)

		cookies := rec.Result().Cookies()
		Expect(cookies).To(HaveLen(1))
		Expect(cookies[0].Secure).To(BeTrue())
	})

	It("does not set Secure flag on the cookie when not configured", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		cookies := rec.Result().Cookies()
		Expect(cookies).To(HaveLen(1))
		Expect(cookies[0].Secure).To(BeFalse())
	})
})

var _ = Describe("resolveLocale", func() {
	supported := []string{"en", "de"}
	matcher := language.NewMatcher([]language.Tag{language.English, language.German})

	It("resolves from cookie", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "locale", Value: "de"})

		Expect(resolveLocale(req, supported, matcher)).To(Equal("de"))
	})

	It("resolves from Accept-Language when no cookie", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Language", "de-DE,de;q=0.9")

		Expect(resolveLocale(req, supported, matcher)).To(Equal("de"))
	})

	It("falls back to en when no cookie and no Accept-Language", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		Expect(resolveLocale(req, supported, matcher)).To(Equal("en"))
	})

	It("rejects unsupported cookie and falls through to Accept-Language", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "locale", Value: "fr"})
		req.Header.Set("Accept-Language", "de-DE,de;q=0.9")

		Expect(resolveLocale(req, supported, matcher)).To(Equal("de"))
	})

	It("falls back to en when both cookie and Accept-Language are unsupported", func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "locale", Value: "fr"})
		req.Header.Set("Accept-Language", "fr-FR,fr;q=0.9")

		Expect(resolveLocale(req, supported, matcher)).To(Equal("en"))
	})
})

var _ = Describe("isValidLocale", func() {
	supported := []string{"en", "de"}

	DescribeTable("validates locale codes",
		func(locale string, expected bool) {
			Expect(isValidLocale(locale, supported)).To(Equal(expected))
		},
		Entry("valid en", "en", true),
		Entry("valid de", "de", true),
		Entry("invalid fr", "fr", false),
		Entry("invalid empty", "", false),
		Entry("valid en uppercase", "EN", true),
		Entry("valid de mixed case", "De", true),
	)
})
