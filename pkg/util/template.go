package util

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/util/validation"
)

var (
	errInvalidLabelKey   = errors.New("invalid label key")
	errInvalidLabelValue = errors.New("invalid label value")
	errTemplateRender    = errors.New("failed to render template")
)

var safeFuncMap = template.FuncMap{
	"default": func(defaultVal, val string) string {
		if val == "" {
			return defaultVal
		}

		return val
	},
	"lower":   strings.ToLower,
	"upper":   strings.ToUpper,
	"trim":    strings.TrimSpace,
	"replace": func(old, replacement, s string) string { return strings.ReplaceAll(s, old, replacement) },
	"trunc": func(length int, s string) string {
		if length < 0 {
			return s
		}

		if len(s) > length {
			return s[:length]
		}

		return s
	},
}

func RenderPodLabels(tmpl, vars map[string]string) (map[string]string, error) {
	rendered := make(map[string]string, len(tmpl))

	for key, value := range tmpl {
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return nil, fmt.Errorf("%w %q: %s", errInvalidLabelKey, key, strings.Join(errs, ", "))
		}

		renderedValue, err := renderTemplate(value, vars)
		if err != nil {
			return nil, fmt.Errorf("%w for key %q: %w", errTemplateRender, key, err)
		}

		if errs := validation.IsValidLabelValue(renderedValue); len(errs) > 0 {
			return nil, fmt.Errorf("%w for key %q: %s", errInvalidLabelValue, key, strings.Join(errs, ", "))
		}

		rendered[key] = renderedValue
	}

	return rendered, nil
}

func renderTemplate(tmpl string, vars map[string]string) (string, error) {
	t, err := template.New("podlabel").
		Option("missingkey=error").
		Funcs(safeFuncMap).
		Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("execution error: %w", err)
	}

	return buf.String(), nil
}

func MergeRenderedPodLabels(podLabels, tmpl, vars map[string]string) error {
	rendered, err := RenderPodLabels(tmpl, vars)
	if err != nil {
		return err
	}

	for k, v := range rendered {
		if _, ok := podLabels[k]; !ok {
			podLabels[k] = v
		}
	}

	return nil
}
