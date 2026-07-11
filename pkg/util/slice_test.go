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

var _ = Describe("ContainsAll", func() {
	It("returns true when a contains all elements of b", func() {
		a := []string{"renovate", "production", "backend"}
		b := []string{"renovate", "production"}

		Expect(ContainsAll(a, b)).To(BeTrue())
	})

	It("returns false when a is missing an element from b", func() {
		a := []string{"renovate"}
		b := []string{"renovate", "production"}

		Expect(ContainsAll(a, b)).To(BeFalse())
	})

	It("returns true when b is empty", func() {
		a := []string{"renovate", "production"}
		b := []string{}

		Expect(ContainsAll(a, b)).To(BeTrue())
	})

	It("returns true when both slices are empty", func() {
		a := []string{}
		b := []string{}

		Expect(ContainsAll(a, b)).To(BeTrue())
	})

	It("returns false when a is empty but b is not", func() {
		a := []string{}
		b := []string{"renovate"}

		Expect(ContainsAll(a, b)).To(BeFalse())
	})

	It("works with int slices", func() {
		a := []int{1, 2, 3, 4, 5}
		b := []int{2, 4}

		Expect(ContainsAll(a, b)).To(BeTrue())
	})

	It("handles nil a", func() {
		var a []string

		b := []string{"renovate"}

		Expect(ContainsAll(a, b)).To(BeFalse())
	})

	It("handles nil b", func() {
		a := []string{"renovate"}

		var b []string

		Expect(ContainsAll(a, b)).To(BeTrue())
	})
})
