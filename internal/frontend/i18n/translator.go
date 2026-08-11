package i18n

import (
	"maps"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// Translator wraps a go-i18n localizer and provides convenience methods
// for translating keys with optional template arguments.
type Translator struct {
	localizer *i18n.Localizer
	bundle    *i18n.Bundle
	locale    string
}

// T translates the given message ID with optional template data.
// If the key is not found in the current locale, go-i18n falls back to English.
// When no localizer is available (e.g. in tests), returns the message ID unchanged.
func (t *Translator) T(messageID string, args ...any) string {
	if t.localizer == nil {
		return messageID
	}

	cfg := &i18n.LocalizeConfig{
		MessageID: messageID,
	}

	if len(args) > 0 {
		cfg.TemplateData = args[0]
	}

	msg, err := t.localizer.Localize(cfg)
	if err != nil && msg == "" {
		return messageID
	}

	return msg
}

// TP translates a pluralized message. The pluralCount is passed to go-i18n
// to select the correct plural form ("one", "other", etc.).
// When no localizer is available (e.g. in tests), returns the message ID unchanged.
func (t *Translator) TP(messageID string, pluralCount int, args ...any) string {
	if t.localizer == nil {
		return messageID
	}

	data := map[string]any{
		"Count": pluralCount,
	}

	if len(args) > 0 {
		if m, ok := args[0].(map[string]any); ok {
			maps.Copy(data, m)
		}
	}

	cfg := &i18n.LocalizeConfig{
		MessageID:    messageID,
		PluralCount:  pluralCount,
		TemplateData: data,
	}

	msg, err := t.localizer.Localize(cfg)
	if err != nil && msg == "" {
		return messageID
	}

	return msg
}

// Locale returns the resolved locale code (e.g. "en", "de").
func (t *Translator) Locale() string {
	return t.locale
}

// Localizer returns the underlying go-i18n localizer.
func (t *Translator) Localizer() *i18n.Localizer {
	return t.localizer
}

// Bundle returns the underlying go-i18n bundle.
func (t *Translator) Bundle() *i18n.Bundle {
	return t.bundle
}

// NewTranslator creates a new Translator instance.
func NewTranslator(localizer *i18n.Localizer, bundle *i18n.Bundle, locale string) *Translator {
	return &Translator{
		localizer: localizer,
		bundle:    bundle,
		locale:    locale,
	}
}

func defaultTranslator() *Translator {
	return NewTranslator(nil, nil, "en")
}

// LanguageName returns the native display name for the given locale by
// resolving the "meta.language_name" key from that locale's own translation
// file. Falls back to the locale code if the bundle is unavailable or the
// key is missing.
func (t *Translator) LanguageName(locale string) string {
	if t.bundle == nil {
		return locale
	}

	localizer := i18n.NewLocalizer(t.bundle, locale)

	name, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: "meta.language_name"})
	if err != nil || name == "" {
		return locale
	}

	return name
}
