package renovator

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Renovate Config Merge", func() {
	const baseConfig = `{
		"onboarding": true,
		"prHourlyLimit": 10,
		"platform": "gitea",
		"endpoint": "https://gitea.example.com/api/v1",
		"addLabels": ["renovate", "dependencies"]
	}`

	It("should add new keys from the overlay", func() {
		overlay := []byte(`{"extends": ["config:recommended"]}`)

		merged, err := mergeRenovateConfig([]byte(baseConfig), overlay)
		Expect(err).NotTo(HaveOccurred())
		Expect(merged).To(MatchJSON(`{
			"onboarding": true,
			"prHourlyLimit": 10,
			"platform": "gitea",
			"endpoint": "https://gitea.example.com/api/v1",
			"addLabels": ["renovate", "dependencies"],
			"extends": ["config:recommended"]
		}`))
	})

	It("should overwrite existing scalar values", func() {
		overlay := []byte(`{"onboarding": false, "prHourlyLimit": 20}`)

		merged, err := mergeRenovateConfig([]byte(baseConfig), overlay)
		Expect(err).NotTo(HaveOccurred())
		Expect(merged).To(MatchJSON(`{
			"onboarding": false,
			"prHourlyLimit": 20,
			"platform": "gitea",
			"endpoint": "https://gitea.example.com/api/v1",
			"addLabels": ["renovate", "dependencies"]
		}`))
	})

	It("should replace arrays instead of merging them", func() {
		overlay := []byte(`{"addLabels": ["custom"]}`)

		merged, err := mergeRenovateConfig([]byte(baseConfig), overlay)
		Expect(err).NotTo(HaveOccurred())
		Expect(merged).To(MatchJSON(`{
			"onboarding": true,
			"prHourlyLimit": 10,
			"platform": "gitea",
			"endpoint": "https://gitea.example.com/api/v1",
			"addLabels": ["custom"]
		}`))
	})

	It("should recursively merge nested objects", func() {
		base := []byte(`{"customManager": {"enabled": true, "matchStrings": ["x"]}}`)
		overlay := []byte(`{"customManager": {"enabled": false}}`)

		merged, err := mergeRenovateConfig(base, overlay)
		Expect(err).NotTo(HaveOccurred())
		Expect(merged).To(MatchJSON(`{
			"customManager": {"enabled": false, "matchStrings": ["x"]}
		}`))
	})

	It("should remove keys set to null", func() {
		overlay := []byte(`{"addLabels": null}`)

		merged, err := mergeRenovateConfig([]byte(baseConfig), overlay)
		Expect(err).NotTo(HaveOccurred())
		Expect(merged).To(MatchJSON(`{
			"onboarding": true,
			"prHourlyLimit": 10,
			"platform": "gitea",
			"endpoint": "https://gitea.example.com/api/v1"
		}`))
	})

	It("should reject a non-object overlay", func() {
		_, err := mergeRenovateConfig([]byte(baseConfig), []byte(`["not", "an", "object"]`))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("config must be a JSON object"))
	})

	It("should reject a string overlay", func() {
		_, err := mergeRenovateConfig([]byte(baseConfig), []byte(`"a string"`))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("config must be a JSON object"))
	})

	It("should reject a null overlay", func() {
		_, err := mergeRenovateConfig([]byte(baseConfig), []byte(`null`))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("config must be a JSON object"))
	})
})
