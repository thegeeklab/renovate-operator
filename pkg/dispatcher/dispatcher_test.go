package dispatcher

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config Merging", func() {
	var d *Dispatcher

	BeforeEach(func() {
		d = &Dispatcher{}
	})

	Context("MergeConfig", func() {
		It("should merge renovate base config with job config", func() {
			baseConfig := []byte(`{
				"extends": ["config:base"],
				"timezone": "Europe/Berlin",
				"dependencyDashboard": true,
				"platformAutomerge": true,
				"prHourlyLimit": 5,
				"repositories": []
			}`)

			indexConfig := []byte(`[
				{"repositories": ["org/repo1"]},
				{"repositories": ["org/repo2"]},
				{"repositories": ["org/repo3"]}
			]`)

			result, err := d.MergeConfig(baseConfig, indexConfig, 2)
			Expect(err).NotTo(HaveOccurred())

			var merged map[string]any
			err = json.Unmarshal(result, &merged)
			Expect(err).NotTo(HaveOccurred())

			Expect(merged["repositories"]).To(Equal([]any{"org/repo3"}))
			Expect(merged["timezone"]).To(Equal("Europe/Berlin"))
			Expect(merged["dependencyDashboard"]).To(BeTrue())
			Expect(merged["platformAutomerge"]).To(BeTrue())
			Expect(merged["prHourlyLimit"]).To(Equal(float64(5)))
		})

		It("should return error for invalid base config JSON", func() {
			baseConfig := []byte(`invalid json`)
			jobConfig := []byte(`[{"name": "test"}]`)

			_, err := d.MergeConfig(baseConfig, jobConfig, 0)
			Expect(err).To(HaveOccurred())
		})

		It("should return error for invalid job config JSON", func() {
			baseConfig := []byte(`{"name": "base"}`)
			jobConfig := []byte(`invalid json`)

			_, err := d.MergeConfig(baseConfig, jobConfig, 0)
			Expect(err).To(HaveOccurred())
		})

		It("should return error for out of bounds index", func() {
			baseConfig := []byte(`{"name": "base"}`)
			indexConfig := []byte(`[{"name": "test"}]`)

			_, err := d.MergeConfig(baseConfig, indexConfig, 1)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("index out of bounds: 1"))
		})

		It("should preserve non-overridden fields from base config", func() {
			baseConfig := []byte(`{"name": "base", "unchanged": "value", "toOverride": "old"}`)
			jobConfig := []byte(`[{"name": "new", "toOverride": "new"}]`)

			result, err := d.MergeConfig(baseConfig, jobConfig, 0)
			Expect(err).NotTo(HaveOccurred())

			var merged map[string]any
			err = json.Unmarshal(result, &merged)
			Expect(err).NotTo(HaveOccurred())
			Expect(merged["name"]).To(Equal("new"))
			Expect(merged["unchanged"]).To(Equal("value"))
			Expect(merged["toOverride"]).To(Equal("new"))
		})
	})
})
