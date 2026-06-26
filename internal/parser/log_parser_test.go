package parser

import (
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thegeeklab/renovate-operator/internal/parser/fixtures"
)

var _ = Describe("LogParser", func() {
	Describe("ParseRenovateLogs", func() {
		It("returns empty result for empty logs", func() {
			result := ParseRenovateLogs("")
			Expect(result.HasIssues).To(BeFalse())
			Expect(result.PRActivity).To(BeNil())
			Expect(result.LogIssues).To(BeNil())
			Expect(result.Lines).To(BeEmpty())
		})

		It("returns empty result for non-JSON logs", func() {
			logs := strings.Join([]string{
				"INFO: Renovate started",
				"DEBUG: Using RE2 regex engine",
				"DEBUG: Parsing configs",
			}, "\n")

			result := ParseRenovateLogs(logs)
			Expect(result.HasIssues).To(BeFalse())
			Expect(result.PRActivity).To(BeNil())
			Expect(result.LogIssues).To(BeNil())
			Expect(result.Lines).To(BeEmpty())
		})

		It("detects warnings and errors", func() {
			result := ParseRenovateLogs(fixtures.WarnAndError)
			Expect(result.HasIssues).To(BeTrue())
			Expect(result.LogIssues).NotTo(BeNil())
			Expect(result.LogIssues.WarnCount).To(Equal(1))
			Expect(result.LogIssues.ErrorCount).To(Equal(1))
			Expect(result.LogIssues.Issues).To(HaveLen(2))
			Expect(result.LogIssues.Issues[0].Level).To(Equal(40))
			Expect(result.LogIssues.Issues[0].Message).To(Equal("Configuration warning"))
			Expect(result.LogIssues.Issues[1].Level).To(Equal(50))
			Expect(result.LogIssues.Truncated).To(BeFalse())
		})

		It("truncates issues when exceeding max", func() {
			var lines []string

			for i := range 25 {
				lines = append(lines, `{"level":40,"msg":"Warning `+strings.Repeat("x", i)+`"}`)
			}

			result := ParseRenovateLogs(strings.Join(lines, "\n"))
			Expect(result.LogIssues).NotTo(BeNil())
			Expect(result.LogIssues.Issues).To(HaveLen(MaxLogIssues))
			Expect(result.LogIssues.Truncated).To(BeTrue())
		})

		It("deduplicates issue messages", func() {
			result := ParseRenovateLogs(fixtures.DuplicateWarnings)
			Expect(result.LogIssues).NotTo(BeNil())
			Expect(result.LogIssues.WarnCount).To(Equal(3))
			Expect(result.LogIssues.Issues).To(HaveLen(1))
		})

		DescribeTable(
			"parses repository finished status",
			func(log, expected string) {
				result := ParseRenovateLogs(log)
				Expect(result.RenovateResultStatus).To(Equal(expected))
			},
			Entry("disabled-by-config", fixtures.RepoFinishedDisabledByConfig, "Disabled"),
			Entry("disabled-closed-onboarding", fixtures.RepoFinishedDisabledClosedOnboarding, "Onboarding Closed"),
			Entry("disabled-no-config", fixtures.RepoFinishedDisabledNoConfig, "No Config"),
			Entry("onboarding status", fixtures.RepoFinishedOnboarding, "Onboarding"),
			Entry("unknown empty", fixtures.RepoFinishedUnknown, "Unknown"),
			Entry("custom result", fixtures.RepoFinishedDone, "done"),
		)

		It("extracts PR created activity", func() {
			result := ParseRenovateLogs(fixtures.PRCreated)
			Expect(result.PRActivity).NotTo(BeNil())
			Expect(result.PRActivity.Created).To(Equal(1))
			Expect(result.PRActivity.PRs).To(HaveLen(1))
			Expect(result.PRActivity.PRs[0].Branch).To(Equal("renovate/lodash-4.x"))
			Expect(result.PRActivity.PRs[0].Number).To(Equal(42))
			Expect(result.PRActivity.PRs[0].Action).To(Equal(PRActionCreated))
			Expect(result.PRActivity.PRs[0].Title).To(Equal("Update dependency lodash to v4.17.21"))
		})

		It("extracts PR updated activity", func() {
			result := ParseRenovateLogs(fixtures.PRUpdated)
			Expect(result.PRActivity).NotTo(BeNil())
			Expect(result.PRActivity.Updated).To(Equal(1))
			Expect(result.PRActivity.PRs).To(HaveLen(1))
			Expect(result.PRActivity.PRs[0].Number).To(Equal(99))
		})

		It("extracts PR automerged activity", func() {
			result := ParseRenovateLogs(fixtures.PRAutomerged)
			Expect(result.PRActivity).NotTo(BeNil())
			Expect(result.PRActivity.Automerged).To(Equal(1))
			Expect(result.PRActivity.PRs).To(HaveLen(1))
			Expect(result.PRActivity.PRs[0].Action).To(Equal(PRActionAutomerged))
			Expect(result.PRActivity.PRs[0].Number).To(Equal(55))
		})

		It("extracts PR unchanged activity", func() {
			result := ParseRenovateLogs(fixtures.PRUnchanged)
			Expect(result.PRActivity).NotTo(BeNil())
			Expect(result.PRActivity.Unchanged).To(Equal(1))
			Expect(result.PRActivity.PRs).To(HaveLen(1))
			Expect(result.PRActivity.PRs[0].Number).To(Equal(12))
			Expect(result.PRActivity.PRs[0].Action).To(Equal(PRActionUnchanged))
		})

		It("processes branches info extended", func() {
			result := ParseRenovateLogs(fixtures.BranchesInfoExtended)
			Expect(result.PRActivity).NotTo(BeNil())
			Expect(result.PRActivity.NeedsApproval).To(Equal(1))
			Expect(result.PRActivity.Unchanged).To(Equal(1))
			Expect(result.PRActivity.PRs).To(HaveLen(2))
		})

		It("caps PR details at maximum", func() {
			var lines []string

			for i := range 120 {
				branch := "renovate/branch-" + strings.Repeat("x", i%10) + "-" + strconv.Itoa(i)
				lines = append(lines, `{"level":30,"msg":"Creating PR","branch":"`+branch+`","title":"PR `+strconv.Itoa(i)+`"}`)
			}

			result := ParseRenovateLogs(strings.Join(lines, "\n"))
			Expect(result.PRActivity).NotTo(BeNil())
			Expect(result.PRActivity.PRs).To(HaveLen(MaxPRDetails))
			Expect(result.PRActivity.Truncated).To(BeTrue())
		})

		It("formats log lines with correct levels", func() {
			result := ParseRenovateLogs(fixtures.MixedLevels)
			Expect(result.Lines).To(HaveLen(4))
			Expect(result.Lines[0].LevelLabel()).To(Equal("INFO"))
			Expect(result.Lines[0].Class).To(Equal("text-white"))
			Expect(result.Lines[0].Message).To(Equal("Info message"))
			Expect(result.Lines[1].LevelLabel()).To(Equal("WARN"))
			Expect(result.Lines[1].Class).To(Equal("text-yellow-400"))
			Expect(result.Lines[1].Message).To(Equal("Warn message"))
			Expect(result.Lines[2].LevelLabel()).To(Equal("ERROR"))
			Expect(result.Lines[2].Class).To(Equal("text-red-500 font-bold"))
			Expect(result.Lines[2].Message).To(Equal("Error message"))
			Expect(result.Lines[3].LevelLabel()).To(Equal("LOG"))
			Expect(result.Lines[3].Class).To(Equal("text-gray-500"))
		})

		It("escapes HTML in messages", func() {
			result := ParseRenovateLogs(fixtures.HTMLInMessage)
			Expect(result.Lines).To(HaveLen(1))
			Expect(result.Lines[0].Message).NotTo(ContainSubstring("<script>"))
			Expect(result.Lines[0].Message).To(ContainSubstring("&lt;script&gt;"))
		})

		It("handles mixed JSON and non-JSON lines", func() {
			result := ParseRenovateLogs(fixtures.MixedJSONAndPlain)
			Expect(result.Lines).To(HaveLen(3))
			Expect(result.Lines[0].LevelLabel()).To(Equal("INFO"))
			Expect(result.Lines[0].Message).To(Equal("Info message"))
			Expect(result.Lines[1].LevelLabel()).To(Equal("LOG"))
			Expect(result.Lines[1].Message).To(ContainSubstring("this is not json"))
			Expect(result.Lines[2].LevelLabel()).To(Equal("WARN"))
			Expect(result.Lines[2].Message).To(Equal("Warn message"))
		})

		It("sorts PRs by action priority then branch name", func() {
			result := ParseRenovateLogs(fixtures.SortedPRs)
			Expect(result.PRActivity).NotTo(BeNil())
			Expect(result.PRActivity.PRs).To(HaveLen(3))
			Expect(result.PRActivity.PRs[0].Action).To(Equal(PRActionAutomerged))
			Expect(result.PRActivity.PRs[0].Branch).To(Equal("renovate/a-package"))
			Expect(result.PRActivity.PRs[1].Action).To(Equal(PRActionCreated))
			Expect(result.PRActivity.PRs[1].Branch).To(Equal("renovate/b-package"))
			Expect(result.PRActivity.PRs[2].Action).To(Equal(PRActionCreated))
			Expect(result.PRActivity.PRs[2].Branch).To(Equal("renovate/c-package"))
		})

		It("extracts PR URLs from git push remote messages", func() {
			result := ParseRenovateLogs(fixtures.PRURLsFromGitPush)
			Expect(result.PRActivity).NotTo(BeNil())
			Expect(result.PRActivity.PRs).To(HaveLen(2))

			var pr1, pr2 PRDetail

			for _, pr := range result.PRActivity.PRs {
				if pr.Branch == "renovate/lodash-4.x" {
					pr1 = pr
				} else {
					pr2 = pr
				}
			}

			Expect(pr1.URL).To(Equal("https://github.com/org/repo/pull/42"))
			Expect(pr1.Number).To(Equal(42))
			Expect(pr2.URL).To(Equal("https://gitlab.com/org/repo/-/merge_requests/99"))
			Expect(pr2.Number).To(Equal(99))
		})
	})

	Describe("LevelLabel", func() {
		It("returns correct labels for all levels", func() {
			Expect(LevelLabel(LogLevelTrace)).To(Equal("TRACE"))
			Expect(LevelLabel(LogLevelDebug)).To(Equal("DEBUG"))
			Expect(LevelLabel(LogLevelInfo)).To(Equal("INFO"))
			Expect(LevelLabel(LogLevelWarn)).To(Equal("WARN"))
			Expect(LevelLabel(LogLevelError)).To(Equal("ERROR"))
			Expect(LevelLabel(LogLevelFatal)).To(Equal("FATAL"))
			Expect(LevelLabel(LogLevel(99))).To(Equal("LOG"))
		})
	})

	Describe("isNDJSON", func() {
		It("returns true for valid NDJSON", func() {
			logs := `{"level":30,"msg":"test"}
{"level":40,"msg":"warning"}`
			Expect(isNDJSON(logs)).To(BeTrue())
		})

		It("returns false for plain text", func() {
			logs := "INFO: Renovate started\nDEBUG: Parsing configs"
			Expect(isNDJSON(logs)).To(BeFalse())
		})

		It("returns false for empty string", func() {
			Expect(isNDJSON("")).To(BeFalse())
		})

		It("skips empty lines", func() {
			logs := "\n\n{\"level\":30,\"msg\":\"test\"}\n\n"
			Expect(isNDJSON(logs)).To(BeTrue())
		})

		It("returns false for invalid JSON", func() {
			logs := "{not valid json}"
			Expect(isNDJSON(logs)).To(BeFalse())
		})
	})
})
