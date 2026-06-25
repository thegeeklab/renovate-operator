package sanitize

import (
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("sanitize helpers", func() {
	Describe("BoolAttr", func() {
		It("returns true for true", func() {
			Expect(BoolAttr(true)).To(Equal("true"))
		})

		It("returns false for false", func() {
			Expect(BoolAttr(false)).To(Equal("false"))
		})
	})

	Describe("QueryEscape", func() {
		DescribeTable(
			"percent-encodes special characters",
			func(input, want string) {
				got := QueryEscape(input)
				Expect(got).To(Equal(want))
				_, err := url.ParseQuery("x=" + got)
				Expect(err).NotTo(HaveOccurred(), "output must be re-parseable by net/url")
			},
			Entry("simple", "simple", "simple"),
			Entry("space", "with space", "with+space"),
			Entry("ampersand and equals", `a&b=c`, "a%26b%3Dc"),
			Entry("slash", "a/b", "a%2Fb"),
			Entry("plus", "a+b", "a%2Bb"),
			Entry("html", "<script>", "%3Cscript%3E"),
			Entry("injection", "name\";alert(1)", "name%22%3Balert%281%29"),
		)
	})

	Describe("GitreposURL", func() {
		It("builds a base URL without sort/order parameters", func() {
			Expect(GitreposURL("ns-1", "uid-2")).
				To(Equal("/gitrepos?namespace=ns-1&renovator=uid-2"))
		})

		It("escapes user-controlled segments", func() {
			got := GitreposURL("ns with space", "uid&with=special")
			Expect(got).NotTo(ContainSubstring(" "))
			Expect(got).NotTo(ContainSubstring("ns with space"))
		})
	})

	Describe("GitrepoURL", func() {
		It("builds a URL with namespace and name", func() {
			Expect(GitrepoURL("ns", "name")).To(Equal("/gitrepo?namespace=ns&name=name"))
		})

		It("escapes user-controlled name", func() {
			got := GitrepoURL("ns", `name";alert(1)`)
			Expect(got).NotTo(ContainSubstring(`";alert`))
		})
	})

	Describe("JobLogsURL", func() {
		It("builds a URL with namespace, runner, and job", func() {
			Expect(JobLogsURL("ns", "runner", "job")).
				To(Equal("/joblogs?namespace=ns&runner=runner&job=job"))
		})
	})

	Describe("PersistKey", func() {
		It("builds a key from namespace and name", func() {
			Expect(PersistKey("ns", "name")).To(Equal("repo-ns-name"))
		})
	})

	Describe("RenovatorPersistKey", func() {
		It("builds a key from the renovator name", func() {
			Expect(RenovatorPersistKey("simple")).To(Equal("renovator-simple"))
		})
	})

	Describe("SortFieldPersistKey", func() {
		It("builds a key from the renovator name", func() {
			Expect(SortFieldPersistKey("my-renovator")).To(Equal("sort-field-my-renovator"))
		})
	})

	Describe("SortOrderPersistKey", func() {
		It("builds a key from the renovator name", func() {
			Expect(SortOrderPersistKey("my-renovator")).To(Equal("sort-order-my-renovator"))
		})
	})
})

var _ = Describe("sanitize regression cases", func() {
	It("QueryEscape output is always safe to drop into a query string", func() {
		dangerous := []string{`"`, `<`, `>`, `&`, `=`, `#`, ` `, `?`}
		for _, in := range dangerous {
			got := QueryEscape(in)
			for _, raw := range dangerous {
				Expect(got).NotTo(ContainSubstring(raw),
					"input %q produced %q containing raw %q", in, got, raw)
			}
		}
	})
})
