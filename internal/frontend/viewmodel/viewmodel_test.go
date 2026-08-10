package viewmodel

import (
	"context"

	"github.com/thegeeklab/renovate-operator/internal/parser"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Status", func() {
	Describe("Label", func() {
		DescribeTable(
			"returns the human-readable label",
			func(s Status, want string) {
				Expect(s.Label()).To(Equal(want))
			},
			Entry("Succeeded", StatusSucceeded, "Succeeded"),
			Entry("Running", StatusRunning, "Running"),
			Entry("Failed", StatusFailed, "Failed"),
			Entry("Unknown", StatusUnknown, "Unknown"),
			Entry("unknown value renders as Unknown", Status("nonsense"), "Unknown"),
		)
	})

	Describe("TranslatedLabel", func() {
		DescribeTable(
			"maps each status to the correct i18n key",
			func(s Status, wantKey string) {
				ctx := context.Background()
				result := s.TranslatedLabel(ctx)
				Expect(result).To(Equal(wantKey))
			},
			Entry("Succeeded", StatusSucceeded, "status.succeeded"),
			Entry("Running", StatusRunning, "status.running"),
			Entry("Failed", StatusFailed, "status.failed"),
			Entry("Unknown", StatusUnknown, "status.unknown"),
			Entry("unknown value", Status("nonsense"), "status.unknown"),
		)
	})

	Describe("BadgeClass", func() {
		It("returns a non-empty class for every known status", func() {
			for _, s := range []Status{StatusSucceeded, StatusRunning, StatusFailed, StatusUnknown} {
				Expect(s.BadgeClass()).NotTo(BeEmpty(), "status %q should have a badge class", s)
			}
		})
	})

	Describe("LeftBorderClass", func() {
		It("returns a non-empty class for every known status", func() {
			for _, s := range []Status{StatusSucceeded, StatusRunning, StatusFailed, StatusUnknown} {
				Expect(s.LeftBorderClass()).NotTo(BeEmpty(), "status %q should have a left-border class", s)
			}
		})
	})
})

var _ = Describe("GitRepoFieldLabel", func() {
	Describe("TranslatedLabel", func() {
		DescribeTable(
			"maps each field label to the correct i18n key",
			func(f GitRepoFieldLabel, wantKey string) {
				ctx := context.Background()
				result := f.TranslatedLabel(ctx)
				Expect(result).To(Equal(wantKey))
			},
			Entry("Name", GitRepoFieldName, "sort.name"),
			Entry("Created", GitRepoFieldCreated, "sort.created"),
			Entry("Last run", GitRepoFieldLastRun, "sort.last_run"),
		)
	})
})

var _ = Describe("TranslatedFormatCount", func() {
	It("delegates to the translator for singular", func() {
		ctx := context.Background()
		result := TranslatedFormatCount(ctx, 1, "badge.error_singular", "badge.error_plural")
		Expect(result).To(ContainSubstring("badge.error_singular"))
	})

	It("delegates to the translator for plural", func() {
		ctx := context.Background()
		result := TranslatedFormatCount(ctx, 5, "badge.warning_singular", "badge.warning_plural")
		Expect(result).To(ContainSubstring("badge.warning_plural"))
	})
})

var _ = Describe("TranslatedIssueSummaryText", func() {
	ctx := context.Background()

	It("returns empty string for nil issues", func() {
		Expect(TranslatedIssueSummaryText(ctx, nil)).To(Equal(""))
	})

	It("returns empty string for issues with zero counts", func() {
		issues := &parser.LogIssues{}
		Expect(TranslatedIssueSummaryText(ctx, issues)).To(Equal(""))
	})

	It("renders error count only", func() {
		issues := &parser.LogIssues{ErrorCount: 2}
		result := TranslatedIssueSummaryText(ctx, issues)
		Expect(result).To(ContainSubstring("badge.error_plural"))
		Expect(result).NotTo(ContainSubstring("warn"))
	})

	It("renders warning count only", func() {
		issues := &parser.LogIssues{WarnCount: 3}
		result := TranslatedIssueSummaryText(ctx, issues)
		Expect(result).To(ContainSubstring("badge.warning_plural"))
		Expect(result).NotTo(ContainSubstring("error"))
	})

	It("renders both error and warning counts", func() {
		issues := &parser.LogIssues{ErrorCount: 2, WarnCount: 4}
		result := TranslatedIssueSummaryText(ctx, issues)
		Expect(result).To(ContainSubstring("badge.error_plural"))
		Expect(result).To(ContainSubstring("badge.warning_plural"))
	})
})

var _ = Describe("TranslatedPRActivitySummary", func() {
	ctx := context.Background()

	It("returns empty string for nil activity", func() {
		Expect(TranslatedPRActivitySummary(ctx, nil)).To(Equal(""))
	})

	It("renders automerged count", func() {
		activity := &parser.PRActivity{Automerged: 3}
		result := TranslatedPRActivitySummary(ctx, activity)
		Expect(result).To(ContainSubstring("badge.automerged_plural"))
	})

	It("renders created count", func() {
		activity := &parser.PRActivity{Created: 2}
		result := TranslatedPRActivitySummary(ctx, activity)
		Expect(result).To(ContainSubstring("badge.created_plural"))
	})

	It("renders updated count", func() {
		activity := &parser.PRActivity{Updated: 1}
		result := TranslatedPRActivitySummary(ctx, activity)
		Expect(result).To(ContainSubstring("badge.updated_singular"))
	})

	It("renders needs approval count", func() {
		activity := &parser.PRActivity{NeedsApproval: 4}
		result := TranslatedPRActivitySummary(ctx, activity)
		Expect(result).To(ContainSubstring("badge.needs_approval_plural"))
	})

	It("renders unchanged count", func() {
		activity := &parser.PRActivity{Unchanged: 5}
		result := TranslatedPRActivitySummary(ctx, activity)
		Expect(result).To(ContainSubstring("badge.unchanged_plural"))
	})

	It("renders a combination of multiple activity types", func() {
		activity := &parser.PRActivity{
			Automerged:    2,
			Created:       3,
			NeedsApproval: 1,
		}
		result := TranslatedPRActivitySummary(ctx, activity)
		Expect(result).To(ContainSubstring("badge.automerged_plural"))
		Expect(result).To(ContainSubstring("badge.created_plural"))
		Expect(result).To(ContainSubstring("badge.needs_approval_singular"))
	})
})
