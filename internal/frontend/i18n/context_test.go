package i18n

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

var _ = Describe("Context", func() {
	It("round-trips a translator via NewContext and FromContext", func() {
		bundle := NewBundle()
		localizer := goi18n.NewLocalizer(bundle, "de", "en")
		tr := newTranslator(localizer, bundle, "de")

		ctx := NewContext(context.Background(), tr)
		got := FromContext(ctx)

		Expect(got).To(Equal(tr))
		Expect(got.Locale()).To(Equal("de"))
	})

	It("returns a default translator when none is stored in context", func() {
		tr := FromContext(context.Background())

		Expect(tr).NotTo(BeNil())
		Expect(tr.Locale()).To(Equal("en"))
	})

	It("returns a default translator when nil is stored in context", func() {
		ctx := context.WithValue(context.Background(), ctxKey{}, (*Translator)(nil))
		tr := FromContext(ctx)

		Expect(tr).NotTo(BeNil())
		Expect(tr.Locale()).To(Equal("en"))
	})
})
