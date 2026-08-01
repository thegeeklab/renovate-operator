package view

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("getPRBadgeColorClasses", func() {
	It("returns yellow classes when PRs need approval", func() {
		classes := getPRBadgeColorClasses(5, 2, 1)
		Expect(classes).To(ContainSubstring("bg-yellow-50"))
		Expect(classes).To(ContainSubstring("text-yellow-700"))
	})

	It("returns yellow classes even when only some PRs need approval", func() {
		classes := getPRBadgeColorClasses(3, 1, 0)
		Expect(classes).To(ContainSubstring("bg-yellow-50"))
	})

	It("returns gray classes when all PRs are unchanged", func() {
		classes := getPRBadgeColorClasses(4, 0, 4)
		Expect(classes).To(ContainSubstring("bg-gray-50"))
		Expect(classes).To(ContainSubstring("text-gray-600"))
	})

	It("returns gray classes only when all PRs are unchanged and there is at least one", func() {
		classes := getPRBadgeColorClasses(3, 0, 1)
		Expect(classes).To(ContainSubstring("bg-blue-50"))
	})

	It("returns blue classes when there are active PRs", func() {
		classes := getPRBadgeColorClasses(2, 0, 0)
		Expect(classes).To(ContainSubstring("bg-blue-50"))
		Expect(classes).To(ContainSubstring("text-blue-700"))
	})

	It("returns blue classes when open is zero", func() {
		classes := getPRBadgeColorClasses(0, 0, 0)
		Expect(classes).To(ContainSubstring("bg-blue-50"))
	})
})

var _ = Describe("getWarningBadgeColorClasses", func() {
	It("returns red classes when there are errors", func() {
		classes := getWarningBadgeColorClasses(1)
		Expect(classes).To(ContainSubstring("bg-red-50"))
		Expect(classes).To(ContainSubstring("text-red-700"))
	})

	It("returns yellow classes when there are no errors", func() {
		classes := getWarningBadgeColorClasses(0)
		Expect(classes).To(ContainSubstring("bg-yellow-50"))
		Expect(classes).To(ContainSubstring("text-yellow-700"))
	})
})

var _ = Describe("pluralize", func() {
	It("returns singular for count of 1", func() {
		result := pluralize(1, "PR", "PRs")
		Expect(result).To(Equal("1 PR"))
	})

	It("returns plural for count of 0", func() {
		result := pluralize(0, "error", "errors")
		Expect(result).To(Equal("0 errors"))
	})

	It("returns plural for count greater than 1", func() {
		result := pluralize(5, "warning", "warnings")
		Expect(result).To(Equal("5 warnings"))
	})
})

var _ = Describe("getPRBadgeTooltip", func() {
	It("informs when there is no recent data", func() {
		tooltip := getPRBadgeTooltip(false, 0, 0, 0)
		Expect(tooltip).To(ContainSubstring("No recent Renovate data"))
	})

	It("shows PRs needing approval with singular", func() {
		tooltip := getPRBadgeTooltip(true, 1, 1, 0)
		Expect(tooltip).To(ContainSubstring("1 PR needs approval"))
	})

	It("shows PRs needing approval with additional active", func() {
		tooltip := getPRBadgeTooltip(true, 3, 1, 0)
		Expect(tooltip).To(ContainSubstring("1 PR needs approval"))
		Expect(tooltip).To(ContainSubstring("additional active"))
	})

	It("shows unchanged PRs when all are unchanged", func() {
		tooltip := getPRBadgeTooltip(true, 2, 0, 2)
		Expect(tooltip).To(ContainSubstring("no updates needed"))
	})

	It("shows no open PRs when count is zero", func() {
		tooltip := getPRBadgeTooltip(true, 0, 0, 0)
		Expect(tooltip).To(Equal("No open PRs"))
	})

	It("shows open PRs count", func() {
		tooltip := getPRBadgeTooltip(true, 3, 0, 0)
		Expect(tooltip).To(Equal("3 PRs open"))
	})
})

var _ = Describe("getWarningsBadgeTooltip", func() {
	It("returns empty string when both counts are zero", func() {
		Expect(getWarningsBadgeTooltip(0, 0)).To(Equal(""))
	})

	It("shows error count only", func() {
		tooltip := getWarningsBadgeTooltip(0, 2)
		Expect(tooltip).To(ContainSubstring("2 errors"))
		Expect(tooltip).NotTo(ContainSubstring("warning"))
	})

	It("shows warning count only", func() {
		tooltip := getWarningsBadgeTooltip(3, 0)
		Expect(tooltip).To(ContainSubstring("3 warnings"))
		Expect(tooltip).NotTo(ContainSubstring("error"))
	})

	It("shows both error and warning counts", func() {
		tooltip := getWarningsBadgeTooltip(4, 2)
		Expect(tooltip).To(ContainSubstring("2 errors"))
		Expect(tooltip).To(ContainSubstring("4 warnings"))
	})
})
