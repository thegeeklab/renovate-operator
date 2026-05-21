package renovate

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Renovate Detection", func() {
	Describe("isRenovateContent", func() {
		It("should return true for content with ## Detected Dependencies", func() {
			Expect(IsRenovateContent("## Detected Dependencies")).To(BeTrue())
		})

		It("should return true for content with <!-- rebase-check -->", func() {
			Expect(IsRenovateContent("<!-- rebase-check -->")).To(BeTrue())
		})

		It("should return true for content with <!--renovate-debug:-->", func() {
			Expect(IsRenovateContent("<!--renovate-debug:test-->")).To(BeTrue())
		})

		It("should return true for content with <!-- rebase-all-open-prs -->", func() {
			Expect(IsRenovateContent("<!-- rebase-all-open-prs -->")).To(BeTrue())
		})

		It("should return true for content with <!-- rebase-branch=", func() {
			Expect(IsRenovateContent("<!-- rebase-branch=main -->")).To(BeTrue())
		})

		It("should return true for content with <!-- approve-all-pending-prs -->", func() {
			Expect(IsRenovateContent("<!-- approve-all-pending-prs -->")).To(BeTrue())
		})

		It("should return true for content with <!-- approvePr-branch=", func() {
			Expect(IsRenovateContent("<!-- approvePr-branch=main -->")).To(BeTrue())
		})

		It("should return true for content with <!-- approve-branch=", func() {
			Expect(IsRenovateContent("<!-- approve-branch=main -->")).To(BeTrue())
		})

		It("should return true for content with <!-- recreate-branch=", func() {
			Expect(IsRenovateContent("<!-- recreate-branch=main -->")).To(BeTrue())
		})

		It("should return true for content with <!-- unschedule-branch=", func() {
			Expect(IsRenovateContent("<!-- unschedule-branch=main -->")).To(BeTrue())
		})

		It("should return true for content with <!-- create-config-migration-pr -->", func() {
			Expect(IsRenovateContent("<!-- create-config-migration-pr -->")).To(BeTrue())
		})

		It("should return true for content with <!-- create-all-awaiting-schedule-prs -->", func() {
			Expect(IsRenovateContent("<!-- create-all-awaiting-schedule-prs -->")).To(BeTrue())
		})

		It("should return true for content with <!-- create-all-rate-limited-prs -->", func() {
			Expect(IsRenovateContent("<!-- create-all-rate-limited-prs -->")).To(BeTrue())
		})

		It("should return true for content with <!-- unlimit-branch=", func() {
			Expect(IsRenovateContent("<!-- unlimit-branch=main -->")).To(BeTrue())
		})

		It("should return true for content with <!-- manual job -->", func() {
			Expect(IsRenovateContent("<!-- manual job -->")).To(BeTrue())
		})

		It("should return false for empty content", func() {
			Expect(IsRenovateContent("")).To(BeFalse())
		})

		It("should return false for content without renovate markers", func() {
			Expect(IsRenovateContent("This is a regular PR description")).To(BeFalse())
		})
	})

	Describe("hasCheckboxBeenChecked", func() {
		It("should return true for content with lowercase - [x]", func() {
			Expect(HasCheckboxBeenChecked("- [x] golangci/golangci-lint")).To(BeTrue())
		})

		It("should return true for content with uppercase - [X]", func() {
			Expect(HasCheckboxBeenChecked("- [X] golangci/golangci-lint")).To(BeTrue())
		})

		It("should return false for content with unchecked - [ ]", func() {
			Expect(HasCheckboxBeenChecked("- [ ] golangci/golangci-lint")).To(BeFalse())
		})

		It("should return false for empty content", func() {
			Expect(HasCheckboxBeenChecked("")).To(BeFalse())
		})
	})

	Describe("verifyRenovateDescriptionChange", func() {
		It("should return true for renovate content with checked checkbox", func() {
			content := "## Detected Dependencies\r\n\r\n- [x] golangci/golangci-lint"
			Expect(VerifyRenovateDescriptionChange(content)).To(BeTrue())
		})

		It("should return false for renovate content without checked checkbox", func() {
			content := "## Detected Dependencies\r\n\r\n- [ ] golangci/golangci-lint"
			Expect(VerifyRenovateDescriptionChange(content)).To(BeFalse())
		})

		It("should return false for regular content with checked checkbox", func() {
			content := "Regular PR body\r\n\r\n- [x] some-task"
			Expect(VerifyRenovateDescriptionChange(content)).To(BeFalse())
		})

		It("should return false for empty content", func() {
			Expect(VerifyRenovateDescriptionChange("")).To(BeFalse())
		})
	})
})
