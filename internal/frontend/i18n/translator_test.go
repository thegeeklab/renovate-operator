package i18n

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

var _ = Describe("Translator", func() {
	var bundle *goi18n.Bundle

	BeforeEach(func() {
		bundle = NewBundle()
	})

	Describe("T", func() {
		It("translates a known key from English", func() {
			localizer := goi18n.NewLocalizer(bundle, "en")
			tr := newTranslator(localizer, bundle, "en")

			Expect(tr.T("common.sign_out")).To(Equal("Sign out"))
		})

		It("translates a known key with template data", func() {
			localizer := goi18n.NewLocalizer(bundle, "en")
			tr := newTranslator(localizer, bundle, "en")

			result := tr.T("login.sign_in_with", map[string]string{"Name": "GitHub"})
			Expect(result).To(Equal("Sign in with GitHub"))
		})

		It("falls back to English when key is missing in non-English locale", func() {
			localizer := goi18n.NewLocalizer(bundle, "de", "en")
			tr := newTranslator(localizer, bundle, "de")

			Expect(tr.T("badge.pr_plural")).To(Equal("PRs"))
		})

		It("returns the message ID when localizer is nil", func() {
			tr := defaultTranslator()

			Expect(tr.T("common.sign_out")).To(Equal("common.sign_out"))
		})

		It("returns the message ID when key is completely unknown", func() {
			localizer := goi18n.NewLocalizer(bundle, "en")
			tr := newTranslator(localizer, bundle, "en")

			Expect(tr.T("nonexistent.key")).To(Equal("nonexistent.key"))
		})
	})

	Describe("TP", func() {
		It("returns singular form for count of 1", func() {
			localizer := goi18n.NewLocalizer(bundle, "en")
			tr := newTranslator(localizer, bundle, "en")

			result := tr.TP("badge.pr_needs_approval", 1)
			Expect(result).To(Equal("1 PR needs approval"))
		})

		It("returns plural form for count greater than 1", func() {
			localizer := goi18n.NewLocalizer(bundle, "en")
			tr := newTranslator(localizer, bundle, "en")

			result := tr.TP("badge.pr_needs_approval", 5)
			Expect(result).To(Equal("5 PRs need approval"))
		})

		It("returns the message ID when localizer is nil", func() {
			tr := defaultTranslator()

			Expect(tr.TP("badge.pr_needs_approval", 1)).To(Equal("badge.pr_needs_approval"))
		})

		It("returns the message ID when plural key is unknown", func() {
			localizer := goi18n.NewLocalizer(bundle, "en")
			tr := newTranslator(localizer, bundle, "en")

			Expect(tr.TP("nonexistent.plural", 5)).To(Equal("nonexistent.plural"))
		})
	})

	Describe("FormatCountMsg", func() {
		It("returns singular form for count of 1", func() {
			localizer := goi18n.NewLocalizer(bundle, "en")
			tr := newTranslator(localizer, bundle, "en")

			result := tr.FormatCountMsg(1, "badge.error_singular", "badge.error_plural")
			Expect(result).To(Equal("1 error"))
		})

		It("returns plural form for count of 0", func() {
			localizer := goi18n.NewLocalizer(bundle, "en")
			tr := newTranslator(localizer, bundle, "en")

			result := tr.FormatCountMsg(0, "badge.error_singular", "badge.error_plural")
			Expect(result).To(Equal("0 errors"))
		})

		It("returns plural form for count greater than 1", func() {
			localizer := goi18n.NewLocalizer(bundle, "en")
			tr := newTranslator(localizer, bundle, "en")

			result := tr.FormatCountMsg(5, "badge.warning_singular", "badge.warning_plural")
			Expect(result).To(Equal("5 warnings"))
		})

		It("falls back to key when translator is nil", func() {
			tr := defaultTranslator()

			result := tr.FormatCountMsg(3, "badge.error_singular", "badge.error_plural")
			Expect(result).To(Equal("3 badge.error_plural"))
		})
	})

	Describe("Locale", func() {
		It("returns the locale set on the translator", func() {
			localizer := goi18n.NewLocalizer(bundle, "de", "en")
			tr := newTranslator(localizer, bundle, "de")

			Expect(tr.Locale()).To(Equal("de"))
		})

		It("returns en for the default translator", func() {
			tr := defaultTranslator()

			Expect(tr.Locale()).To(Equal("en"))
		})
	})

	Describe("Bundle", func() {
		It("returns the underlying bundle", func() {
			localizer := goi18n.NewLocalizer(bundle, "en")
			tr := newTranslator(localizer, bundle, "en")

			Expect(tr.Bundle()).To(Equal(bundle))
		})

		It("returns nil for the default translator", func() {
			tr := defaultTranslator()

			Expect(tr.Bundle()).To(BeNil())
		})
	})

	Describe("LanguageName", func() {
		It("returns the native name for a known locale", func() {
			localizer := goi18n.NewLocalizer(bundle, "en")
			tr := newTranslator(localizer, bundle, "en")

			Expect(tr.LanguageName("en")).To(Equal("English"))
			Expect(tr.LanguageName("de")).To(Equal("Deutsch"))
		})

		It("falls back to the locale code when the bundle is nil", func() {
			tr := defaultTranslator()

			Expect(tr.LanguageName("fr")).To(Equal("fr"))
		})

		It("falls back to English when meta.language_name is missing for unknown locale", func() {
			localizer := goi18n.NewLocalizer(bundle, "en")
			tr := newTranslator(localizer, bundle, "en")

			Expect(tr.LanguageName("xx")).To(Equal("English"))
		})
	})
})
