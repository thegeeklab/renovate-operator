package i18n

import (
	"embed"
	"encoding/json"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

const defaultLanguage = "en"

// NewBundle creates a new i18n bundle with all embedded locale files loaded.
func NewBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return bundle
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		_, _ = bundle.LoadMessageFileFS(localeFS, "locales/"+entry.Name())
	}

	return bundle
}

// SupportedLocales returns the list of locale codes for which locale files exist.
func SupportedLocales() []string {
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return []string{defaultLanguage}
	}

	locales := make([]string, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if len(name) > 5 && name[len(name)-5:] == ".json" {
			locales = append(locales, name[:len(name)-5])
		}
	}

	return locales
}

// ClientJSON returns the JSON-encoded translations for the given locale,
// suitable for injection into a <script> tag for client-side use.
// It transforms the go-i18n message format into nested JSON matched on dot-separated
// message IDs, producing a structure like {"common": {"sign_out": "Sign out"}}.
// go-i18n's json.Marshal escapes <, >, & to Unicode sequences by default,
// which prevents </script> breakouts.
func ClientJSON(bundle *i18n.Bundle, locale string) string {
	data, err := localeFS.ReadFile("locales/" + locale + ".json")
	if err != nil {
		data, _ = localeFS.ReadFile("locales/" + defaultLanguage + ".json")
	}

	var messages []struct {
		ID          string          `json:"id"`
		Translation json.RawMessage `json:"translation"`
	}

	if err := json.Unmarshal(data, &messages); err != nil {
		return "{}"
	}

	nested := make(map[string]any)

	for _, msg := range messages {
		keys := strings.Split(msg.ID, ".")
		parent := nested

		for i := 0; i < len(keys)-1; i++ {
			if v, ok := parent[keys[i]]; ok {
				if m, ok2 := v.(map[string]any); ok2 {
					parent = m
				} else {
					m = make(map[string]any)
					parent[keys[i]] = m
					parent = m
				}
			} else {
				m := make(map[string]any)
				parent[keys[i]] = m
				parent = m
			}
		}

		var val any
		if err := json.Unmarshal(msg.Translation, &val); err != nil {
			val = string(msg.Translation)
		}

		parent[keys[len(keys)-1]] = val
	}

	out, err := json.Marshal(nested)
	if err != nil {
		return "{}"
	}

	return string(out)
}
