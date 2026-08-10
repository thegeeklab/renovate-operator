package view

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thegeeklab/renovate-operator/internal/frontend/i18n"
)

func testCtx() context.Context {
	return i18n.NewContext(context.Background(), i18n.FromContext(context.Background()))
}

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
		result := pluralize(testCtx(), 1, "badge.pr_singular", "badge.pr_plural")
		Expect(result).To(ContainSubstring("1"))
		Expect(result).To(ContainSubstring("badge.pr_singular"))
	})

	It("returns plural for count of 0", func() {
		result := pluralize(testCtx(), 0, "badge.error_singular", "badge.error_plural")
		Expect(result).To(ContainSubstring("0"))
		Expect(result).To(ContainSubstring("badge.error_plural"))
	})

	It("returns plural for count greater than 1", func() {
		result := pluralize(testCtx(), 5, "badge.warning_singular", "badge.warning_plural")
		Expect(result).To(ContainSubstring("5"))
		Expect(result).To(ContainSubstring("badge.warning_plural"))
	})
})

var _ = Describe("getPRBadgeTooltip", func() {
	It("informs when there is no recent data", func() {
		tooltip := getPRBadgeTooltip(testCtx(), false, 0, 0, 0)
		Expect(tooltip).To(ContainSubstring("no_recent_data"))
	})

	It("shows PRs needing approval with singular", func() {
		tooltip := getPRBadgeTooltip(testCtx(), true, 1, 1, 0)
		Expect(tooltip).To(ContainSubstring("pr_needs_approval"))
	})

	It("shows PRs needing approval with additional active", func() {
		tooltip := getPRBadgeTooltip(testCtx(), true, 3, 1, 0)
		Expect(tooltip).To(ContainSubstring("pr_needs_approval"))
	})

	It("shows unchanged PRs when all are unchanged", func() {
		tooltip := getPRBadgeTooltip(testCtx(), true, 2, 0, 2)
		Expect(tooltip).To(ContainSubstring("pr_no_updates"))
	})

	It("shows no open PRs when count is zero", func() {
		tooltip := getPRBadgeTooltip(testCtx(), true, 0, 0, 0)
		Expect(tooltip).To(ContainSubstring("no_open_prs"))
	})

	It("shows open PRs count", func() {
		tooltip := getPRBadgeTooltip(testCtx(), true, 3, 0, 0)
		Expect(tooltip).To(ContainSubstring("badge.pr_open"))
	})
})

var _ = Describe("getWarningsBadgeTooltip", func() {
	It("returns empty string when both counts are zero", func() {
		Expect(getWarningsBadgeTooltip(testCtx(), 0, 0)).To(Equal(""))
	})

	It("shows error count only", func() {
		tooltip := getWarningsBadgeTooltip(testCtx(), 0, 2)
		Expect(tooltip).To(ContainSubstring("badge.error_plural"))
		Expect(tooltip).NotTo(ContainSubstring("badge.warning"))
	})

	It("shows warning count only", func() {
		tooltip := getWarningsBadgeTooltip(testCtx(), 3, 0)
		Expect(tooltip).To(ContainSubstring("badge.warning_plural"))
		Expect(tooltip).NotTo(ContainSubstring("error"))
	})

	It("shows both error and warning counts", func() {
		tooltip := getWarningsBadgeTooltip(testCtx(), 4, 2)
		Expect(tooltip).To(ContainSubstring("badge.error_plural"))
		Expect(tooltip).To(ContainSubstring("badge.warning_plural"))
	})
})
