package parser

import (
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
			logs := strings.Join([]string{
				`{"level":30,"msg":"Starting renovation"}`,
				`{"level":40,"msg":"Configuration warning"}`,
				`{"level":50,"msg":"Failed to fetch dependency"}`,
			}, "\n")

			result := ParseRenovateLogs(logs)
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
			logs := strings.Join([]string{
				`{"level":40,"msg":"Same warning"}`,
				`{"level":40,"msg":"Same warning"}`,
				`{"level":40,"msg":"Same warning"}`,
			}, "\n")

			result := ParseRenovateLogs(logs)
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
			Entry("disabled-by-config",
				`{"level":30,"msg":"Repository finished","result":"disabled-by-config"}`,
				"Disabled"),
			Entry("disabled-closed-onboarding",
				`{"level":30,"msg":"Repository finished","result":"disabled-closed-onboarding"}`,
				"Onboarding Closed"),
			Entry("disabled-no-config",
				`{"level":30,"msg":"Repository finished","result":"disabled-no-config"}`,
				"No Config"),
			Entry("onboarding status",
				`{"level":30,"msg":"Repository finished","status":"onboarding"}`,
				"Onboarding"),
			Entry("unknown empty",
				`{"level":30,"msg":"Repository finished"}`,
				"Unknown"),
			Entry("custom result",
				`{"level":30,"msg":"Repository finished","result":"done"}`,
				"done"),
		)

		It("extracts PR created activity", func() {
			logs := strings.Join([]string{
				`{"level":30,"msg":"Creating PR","branch":"renovate/lodash-4.x",` +
					`"title":"Update dependency lodash to v4.17.21"}`,
				`{"level":30,"msg":"PR created","branch":"renovate/lodash-4.x",` +
					`"pr":42,"prTitle":"Update dependency lodash to v4.17.21"}`,
			}, "\n")

			result := ParseRenovateLogs(logs)
			Expect(result.PRActivity).NotTo(BeNil())
			Expect(result.PRActivity.Created).To(Equal(1))
			Expect(result.PRActivity.PRs).To(HaveLen(1))
			Expect(result.PRActivity.PRs[0].Branch).To(Equal("renovate/lodash-4.x"))
			Expect(result.PRActivity.PRs[0].Number).To(Equal(42))
			Expect(result.PRActivity.PRs[0].Action).To(Equal(PRActionCreated))
			Expect(result.PRActivity.PRs[0].Title).To(Equal("Update dependency lodash to v4.17.21"))
		})

		It("extracts PR updated activity", func() {
			logs := strings.Join([]string{
				`{"level":30,"msg":"Updating PR","branch":"renovate/lodash-4.x","title":"Update lodash"}`,
				`{"level":20,"msg":"git push","branch":"renovate/lodash-4.x",` +
					`"result":{"remoteMessages":{"all":["https://github.com/org/repo/pull/99"]}}}`,
			}, "\n")

			result := ParseRenovateLogs(logs)
			Expect(result.PRActivity).NotTo(BeNil())
			Expect(result.PRActivity.Updated).To(Equal(1))
			Expect(result.PRActivity.PRs).To(HaveLen(1))
			Expect(result.PRActivity.PRs[0].Number).To(Equal(99))
		})

		It("extracts PR automerged activity", func() {
			logs := `{"level":30,"msg":"PR automerged","branch":"renovate/lodash-4.x","pr":55,"prTitle":"Auto merge"}`
			result := ParseRenovateLogs(logs)
			Expect(result.PRActivity).NotTo(BeNil())
			Expect(result.PRActivity.Automerged).To(Equal(1))
			Expect(result.PRActivity.PRs).To(HaveLen(1))
			Expect(result.PRActivity.PRs[0].Action).To(Equal(PRActionAutomerged))
			Expect(result.PRActivity.PRs[0].Number).To(Equal(55))
		})

		It("extracts PR unchanged activity", func() {
			logs := `{"level":30,"msg":"Pull Request #12 does not need updating","branch":"renovate/eslint-8.x"}`
			result := ParseRenovateLogs(logs)
			Expect(result.PRActivity).NotTo(BeNil())
			Expect(result.PRActivity.Unchanged).To(Equal(1))
			Expect(result.PRActivity.PRs).To(HaveLen(1))
			Expect(result.PRActivity.PRs[0].Number).To(Equal(12))
			Expect(result.PRActivity.PRs[0].Action).To(Equal(PRActionUnchanged))
		})

		It("processes branches info extended", func() {
			logs := `{"level":20,"msg":"branches info extended","branchesInformation":[` +
				`{"branchName":"renovate/lodash-4.x","prNo":10,"prTitle":"Update lodash","result":"done"},` +
				`{"branchName":"renovate/stale","prNo":5,"prTitle":"Stale PR","result":"already-existed"},` +
				`{"branchName":"renovate/approval","prNo":null,"prTitle":"Needs review","result":"needs-approval"}]}`

			result := ParseRenovateLogs(logs)
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
			logs := strings.Join([]string{
				`{"level":30,"msg":"Info message"}`,
				`{"level":40,"msg":"Warn message"}`,
				`{"level":50,"msg":"Error message"}`,
				"not json at all",
			}, "\n")

			result := ParseRenovateLogs(logs)
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
			logs := `{"level":30,"msg":"<script>alert('xss')</script>"}`
			result := ParseRenovateLogs(logs)
			Expect(result.Lines).To(HaveLen(1))
			Expect(result.Lines[0].Message).NotTo(ContainSubstring("<script>"))
			Expect(result.Lines[0].Message).To(ContainSubstring("&lt;script&gt;"))
		})

		It("handles mixed JSON and non-JSON lines", func() {
			logs := strings.Join([]string{
				`{"level":30,"msg":"Info message"}`,
				"this is not json",
				`{"level":40,"msg":"Warn message"}`,
			}, "\n")

			result := ParseRenovateLogs(logs)
			Expect(result.Lines).To(HaveLen(3))
			Expect(result.Lines[0].LevelLabel()).To(Equal("INFO"))
			Expect(result.Lines[0].Message).To(Equal("Info message"))
			Expect(result.Lines[1].LevelLabel()).To(Equal("LOG"))
			Expect(result.Lines[1].Message).To(ContainSubstring("this is not json"))
			Expect(result.Lines[2].LevelLabel()).To(Equal("WARN"))
			Expect(result.Lines[2].Message).To(Equal("Warn message"))
		})

		It("sorts PRs by action priority then branch name", func() {
			logs := strings.Join([]string{
				`{"level":30,"msg":"Creating PR","branch":"renovate/b-package","title":"B"}`,
				`{"level":30,"msg":"PR automerged","branch":"renovate/a-package","pr":1,"prTitle":"A"}`,
				`{"level":30,"msg":"Creating PR","branch":"renovate/c-package","title":"C"}`,
			}, "\n")

			result := ParseRenovateLogs(logs)
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
			logs := strings.Join([]string{
				`{"level":30,"msg":"Creating PR","branch":"renovate/lodash-4.x",` +
					`"title":"Update dependency lodash to v4.17.21"}`,
				`{"level":30,"msg":"PR created","branch":"renovate/lodash-4.x",` +
					`"pr":42,"prTitle":"Update dependency lodash to v4.17.21"}`,
				`{"level":20,"msg":"git push","branch":"renovate/lodash-4.x",` +
					`"result":{"remoteMessages":{"all":["https://github.com/org/repo/pull/42"]}}}`,
				`{"level":20,"msg":"git push","branch":"renovate/other",` +
					`"result":{"remoteMessages":{"all":["https://gitlab.com/org/repo/-/merge_requests/99"]}}}`,
			}, "\n")

			result := ParseRenovateLogs(logs)
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
