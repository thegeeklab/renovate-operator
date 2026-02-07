package util

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SplitAndTrimString", func() {
	It("should return nil for empty string", func() {
		result := SplitAndTrimString("", ",")
		Expect(result).To(BeNil())
	})

	It("should split string by separator and trim spaces", func() {
		result := SplitAndTrimString("one, two, three", ",")
		Expect(result).To(Equal([]string{"one", "two", "three"}))
	})

	It("should handle multiple spaces around separator", func() {
		result := SplitAndTrimString("one , two , three", ",")
		Expect(result).To(Equal([]string{"one", "two", "three"}))
	})

	It("should handle different separator", func() {
		result := SplitAndTrimString("one| two| three", "|")
		Expect(result).To(Equal([]string{"one", "two", "three"}))
	})

	It("should handle single item", func() {
		result := SplitAndTrimString("single", ",")
		Expect(result).To(Equal([]string{"single"}))
	})

	It("should handle items with leading/trailing spaces", func() {
		result := SplitAndTrimString("  one  ,  two  ,  three  ", ",")
		Expect(result).To(Equal([]string{"one", "two", "three"}))
	})

	It("should handle empty items in middle", func() {
		result := SplitAndTrimString("one,,three", ",")
		Expect(result).To(Equal([]string{"one", "", "three"}))
	})

	It("should handle all empty items", func() {
		result := SplitAndTrimString(",,,", ",")
		Expect(result).To(Equal([]string{"", "", "", ""}))
	})

	It("should handle string with only separator", func() {
		result := SplitAndTrimString(",", ",")
		Expect(result).To(Equal([]string{"", ""}))
	})

	It("should handle multiline separator", func() {
		result := SplitAndTrimString("one\ntwo\nthree", "\n")
		Expect(result).To(Equal([]string{"one", "two", "three"}))
	})
})
