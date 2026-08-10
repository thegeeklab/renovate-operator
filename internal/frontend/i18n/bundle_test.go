package i18n

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var _ = Describe("Bundle", func() {
	var bundle *goi18n.Bundle

	BeforeEach(func() {
		bundle = NewBundle()
	})

	It("contains both en and de language tags", func() {
		tags := bundle.LanguageTags()
		Expect(tags).To(ContainElement(language.English))
		Expect(tags).To(ContainElement(language.German))
	})

	It("can translate English keys", func() {
		localizer := goi18n.NewLocalizer(bundle, "en")
		msg, err := localizer.Localize(&goi18n.LocalizeConfig{MessageID: "common.sign_out"})

		Expect(err).NotTo(HaveOccurred())
		Expect(msg).To(Equal("Sign out"))
	})
})

var _ = Describe("SupportedLocales", func() {
	It("includes en and de", func() {
		locales := SupportedLocales()

		Expect(locales).To(ContainElement("en"))
		Expect(locales).To(ContainElement("de"))
	})

	It("returns at least the embedded locale count", func() {
		locales := SupportedLocales()

		Expect(len(locales)).To(BeNumerically(">=", 2))
	})
})

var _ = Describe("ClientJSON", func() {
	var bundle *goi18n.Bundle

	BeforeEach(func() {
		bundle = NewBundle()
	})

	It("produces valid JSON for the en locale", func() {
		raw := ClientJSON(bundle, "en")

		var result map[string]any

		err := json.Unmarshal([]byte(raw), &result)
		Expect(err).NotTo(HaveOccurred())
	})

	It("produces nested JSON for dot-separated keys", func() {
		raw := ClientJSON(bundle, "en")

		var result map[string]any
		Expect(json.Unmarshal([]byte(raw), &result)).NotTo(HaveOccurred())

		common, ok := result["common"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(common["sign_out"]).To(Equal("Sign out"))
	})

	It("handles plural translations as nested objects", func() {
		raw := ClientJSON(bundle, "en")

		var result map[string]any
		Expect(json.Unmarshal([]byte(raw), &result)).NotTo(HaveOccurred())

		badge, ok := result["badge"].(map[string]any)
		Expect(ok).To(BeTrue())

		prApproval, ok := badge["pr_needs_approval"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(prApproval["one"]).To(Equal("{{.Count}} PR needs approval"))
		Expect(prApproval["other"]).To(Equal("{{.Count}} PRs need approval"))
	})

	It("falls back to en translations for de locale", func() {
		raw := ClientJSON(bundle, "de")

		var result map[string]any
		Expect(json.Unmarshal([]byte(raw), &result)).NotTo(HaveOccurred())
	})

	It("falls back to en translations for a nonexistent locale", func() {
		raw := ClientJSON(bundle, "xx")

		var result map[string]any
		Expect(json.Unmarshal([]byte(raw), &result)).NotTo(HaveOccurred())

		common, ok := result["common"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(common["sign_out"]).To(Equal("Sign out"))
	})
})
