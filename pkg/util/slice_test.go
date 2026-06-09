package util

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("EmptyIfNil", func() {
	It("returns empty slice for nil input", func() {
		var s []string

		result := EmptyIfNil(s)
		Expect(result).NotTo(BeNil())
		Expect(result).To(BeEmpty())
	})

	It("returns the original slice when non-nil", func() {
		s := []string{"a", "b"}

		result := EmptyIfNil(s)
		Expect(result).To(Equal([]string{"a", "b"}))
	})

	It("returns empty non-nil slice as-is", func() {
		s := []int{}

		result := EmptyIfNil(s)
		Expect(result).NotTo(BeNil())
		Expect(result).To(BeEmpty())
	})

	It("works with int slices", func() {
		var s []int

		result := EmptyIfNil(s)
		Expect(result).To(Equal([]int{}))
	})

	It("preserves non-nil struct slices", func() {
		type item struct{ Name string }

		s := []item{{Name: "x"}}

		result := EmptyIfNil(s)
		Expect(result).To(HaveLen(1))
		Expect(result[0].Name).To(Equal("x"))
	})
})
