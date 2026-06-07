package viewmodel

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Status", func() {
	Describe("ParseStatus", func() {
		DescribeTable(
			"maps raw status strings",
			func(input string, want Status) {
				Expect(ParseStatus(input)).To(Equal(want))
			},
			Entry("Succeeded", "Succeeded", StatusSucceeded),
			Entry("Running", "Running", StatusRunning),
			Entry("Failed", "Failed", StatusFailed),
			Entry("empty", "", StatusUnknown),
			Entry("garbage", "garbage", StatusUnknown),
			Entry("trailing space is not Succeeded", "Succeeded ", StatusUnknown),
		)
	})

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
