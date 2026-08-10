package i18n

import (
	"net/http"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/thegeeklab/renovate-operator/internal/frontend/auth"
	"golang.org/x/text/language"
)

const (
	cookieName   = "locale"
	cookieMaxAge = 365 * 24 * 60 * 60 // 1 year in seconds
)

// MiddlewareConfig controls optional behavior of the i18n middleware.
type MiddlewareConfig struct {
	SecureCookies bool
}

// Middleware returns an HTTP middleware that resolves the user's locale
// from the cookie or Accept-Language header, creates a Translator, injects
// it into the request context, and sets the locale cookie + Content-Language
// header on the response.
func Middleware(bundle *i18n.Bundle, cfg MiddlewareConfig) func(http.Handler) http.Handler {
	supported := SupportedLocales()
	supportedTags := make([]language.Tag, len(supported))

	for i, loc := range supported {
		tag, err := language.Parse(loc)
		if err != nil {
			tag = language.English
		}

		supportedTags[i] = tag
	}

	matcher := language.NewMatcher(supportedTags)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			locale := resolveLocale(r, supported, matcher)

			localizer := i18n.NewLocalizer(bundle, locale, defaultLanguage)
			translator := newTranslator(localizer, bundle, locale)

			ctx := NewContext(r.Context(), translator)
			r = r.WithContext(ctx)

			// Skip cookie and Content-Language for API routes: API clients
			// do not render localized HTML, so the cookie is unnecessary.
			if !auth.IsAPIPath(r.URL.Path) {
				w.Header().Set("Content-Language", locale)
				//nolint:gosec // Cookie is intentionally NOT HttpOnly — JS needs to write it for user-initiated locale switching.
				http.SetCookie(w, &http.Cookie{
					Name:     cookieName,
					Value:    locale,
					Path:     "/",
					MaxAge:   int(cookieMaxAge),
					SameSite: http.SameSiteLaxMode,
					Secure:   cfg.SecureCookies,
				})
			}

			next.ServeHTTP(w, r)
		})
	}
}

// resolveLocale determines the best locale from the request.
// Priority: cookie (validated against supported list) > Accept-Language > default.
func resolveLocale(r *http.Request, supported []string, matcher language.Matcher) string {
	if cookie, err := r.Cookie(cookieName); err == nil {
		if isValidLocale(cookie.Value, supported) {
			return cookie.Value
		}
	}

	if locale := resolveAcceptLanguage(r.Header.Get("Accept-Language"), supported, matcher); locale != "" {
		return locale
	}

	return defaultLanguage
}

func resolveAcceptLanguage(header string, supported []string, matcher language.Matcher) string {
	if header == "" {
		return ""
	}

	acceptTags, _, err := language.ParseAcceptLanguage(header)
	if err != nil || len(acceptTags) == 0 {
		return ""
	}

	tag, _, conf := matcher.Match(acceptTags...)
	if conf < language.High {
		return ""
	}

	locale := tag.String()
	if isValidLocale(locale, supported) {
		return locale
	}

	base, _ := tag.Base()
	locale = base.String()

	if isValidLocale(locale, supported) {
		return locale
	}

	return ""
}

func isValidLocale(locale string, supported []string) bool {
	for _, s := range supported {
		if strings.EqualFold(s, locale) {
			return true
		}
	}

	return false
}
