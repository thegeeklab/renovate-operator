package util

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RenderPodLabels", func() {
	It("renders all template variables", func() {
		tmpl := map[string]string{
			"cost-center": "ns-{{ .namespace }}-comp-{{ .renovator }}",
		}
		vars := map[string]string{
			"namespace": "prod",
			"renovator": "my-renovator",
		}

		result, err := RenderPodLabels(tmpl, vars)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]string{
			"cost-center": "ns-prod-comp-my-renovator",
		}))
	})

	It("renders multiple labels", func() {
		tmpl := map[string]string{
			"team":   "{{ .namespace }}",
			"runner": "{{ .runner }}",
		}
		vars := map[string]string{
			"namespace": "staging",
			"runner":    "my-runner",
		}

		result, err := RenderPodLabels(tmpl, vars)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]string{
			"team":   "staging",
			"runner": "my-runner",
		}))
	})

	It("renders partial matches only", func() {
		tmpl := map[string]string{
			"partial": "{{ .namespace }}-name",
		}
		vars := map[string]string{
			"namespace": "prod",
		}

		result, err := RenderPodLabels(tmpl, vars)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]string{
			"partial": "prod-name",
		}))
	})

	It("returns empty map for nil input", func() {
		result, err := RenderPodLabels(nil, map[string]string{"namespace": "prod"})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result).To(BeEmpty())
	})

	It("leaves labels without templates unchanged", func() {
		tmpl := map[string]string{
			"static": "value",
		}
		vars := map[string]string{
			"namespace": "prod",
		}

		result, err := RenderPodLabels(tmpl, vars)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]string{
			"static": "value",
		}))
	})

	It("returns error for invalid label keys", func() {
		tmpl := map[string]string{
			"invalid key with spaces": "value",
		}
		vars := map[string]string{}

		_, err := RenderPodLabels(tmpl, vars)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid label key"))
	})

	It("returns error for invalid label values", func() {
		tmpl := map[string]string{
			"label": "invalid label with spaces",
		}
		vars := map[string]string{}

		_, err := RenderPodLabels(tmpl, vars)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid label value"))
	})

	It("returns error for label values exceeding max length", func() {
		longValue := strings.Repeat("a", 100)
		tmpl := map[string]string{
			"label": longValue,
		}
		vars := map[string]string{}

		_, err := RenderPodLabels(tmpl, vars)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid label value"))
	})

	It("returns error for undefined template variables", func() {
		tmpl := map[string]string{
			"label": "{{ .undefined }}",
		}
		vars := map[string]string{}

		_, err := RenderPodLabels(tmpl, vars)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to render template"))
	})

	It("returns error for malformed template syntax", func() {
		tmpl := map[string]string{
			"label": "{{ .namespace ",
		}
		vars := map[string]string{
			"namespace": "prod",
		}

		_, err := RenderPodLabels(tmpl, vars)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to render template"))
	})

	It("supports the default function", func() {
		tmpl := map[string]string{
			"label": `{{ .discovery | default "none" }}`,
		}
		vars := map[string]string{
			"discovery": "",
		}

		result, err := RenderPodLabels(tmpl, vars)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]string{
			"label": "none",
		}))
	})

	It("supports the lower function", func() {
		tmpl := map[string]string{
			"label": `{{ .namespace | lower }}`,
		}
		vars := map[string]string{
			"namespace": "PROD",
		}

		result, err := RenderPodLabels(tmpl, vars)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]string{
			"label": "prod",
		}))
	})

	It("supports the trunc function", func() {
		tmpl := map[string]string{
			"label": `{{ .namespace | trunc 4 }}`,
		}
		vars := map[string]string{
			"namespace": "production",
		}

		result, err := RenderPodLabels(tmpl, vars)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]string{
			"label": "prod",
		}))
	})
})

var _ = Describe("MergeRenderedPodLabels", func() {
	It("merges rendered labels into existing map", func() {
		podLabels := map[string]string{
			"existing": "value",
		}
		tmpl := map[string]string{
			"new-label": "{{ .namespace }}",
		}
		vars := map[string]string{
			"namespace": "prod",
		}

		err := MergeRenderedPodLabels(podLabels, tmpl, vars)
		Expect(err).NotTo(HaveOccurred())
		Expect(podLabels).To(Equal(map[string]string{
			"existing":  "value",
			"new-label": "prod",
		}))
	})

	It("does not overwrite existing labels", func() {
		podLabels := map[string]string{
			"label": "original",
		}
		tmpl := map[string]string{
			"label": "{{ .namespace }}",
		}
		vars := map[string]string{
			"namespace": "prod",
		}

		err := MergeRenderedPodLabels(podLabels, tmpl, vars)
		Expect(err).NotTo(HaveOccurred())
		Expect(podLabels["label"]).To(Equal("original"))
	})

	It("returns error for invalid template", func() {
		podLabels := map[string]string{}
		tmpl := map[string]string{
			"invalid key with spaces": "value",
		}
		vars := map[string]string{}

		err := MergeRenderedPodLabels(podLabels, tmpl, vars)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid label key"))
	})
})
