package i18n

import (
	"context"
)

type ctxKey struct{}

// NewContext returns a new context that carries the given translator.
func NewContext(ctx context.Context, t *Translator) context.Context {
	return context.WithValue(ctx, ctxKey{}, t)
}

// FromContext returns the translator stored in the context, or a default
// English translator if none is present.
func FromContext(ctx context.Context) *Translator {
	if t, ok := ctx.Value(ctxKey{}).(*Translator); ok && t != nil {
		return t
	}

	return defaultTranslator()
}
