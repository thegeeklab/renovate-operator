package sanitize

import (
	"encoding/json"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("sanitize helpers", func() {
	Describe("JSString", func() {
		DescribeTable(
			"produces a valid JS string literal",
			func(input, want string) {
				Expect(JSString(input)).To(Equal(want))
			},
			Entry("empty", "", `""`),
			Entry("plain", "foo", `"foo"`),
			Entry("double-quote", `foo"bar`, `"foo\"bar"`),
			Entry("single-quote", `foo'bar`, `"foo'bar"`),
			Entry("backslash", `foo\bar`, `"foo\\bar"`),
			Entry("newline", "foo\nbar", `"foo\nbar"`),
			Entry("angle-bracket", "foo<bar>baz", `"foo\u003cbar\u003ebaz"`),
			Entry("html-injection", `"><script>alert(1)</script>`, `"\"\u003e\u003cscript\u003ealert(1)\u003c/script\u003e"`),
			Entry("unicode-separator", "foo\u2028bar", `"foo\u2028bar"`),
		)

		DescribeTable(
			"always wraps in double quotes and never emits raw newlines",
			func(input string) {
				got := JSString(input)
				Expect(got).NotTo(BeEmpty())
				Expect(string(got[0])).To(Equal(`"`))
				Expect(string(got[len(got)-1])).To(Equal(`"`))
				Expect(got).NotTo(ContainSubstring("\n"))
				Expect(got).NotTo(ContainSubstring("\r"))
			},
			Entry("simple", "simple"),
			Entry("quotes", `with "double" and 'single' quotes`),
			Entry("newline", "with\nnewline"),
			Entry("tab", "with\ttab"),
			Entry("tag", "with<tag>"),
			Entry("script-tag", `</script><img src=x onerror=alert(1)>`),
			Entry("unicode separator", "unicode\u2028separator"),
			Entry("empty", ""),
		)
	})

	Describe("JSValue", func() {
		DescribeTable(
			"serializes values as JS literals",
			func(input any, want string) {
				Expect(JSValue(input)).To(Equal(want))
			},
			Entry("string", "foo", `"foo"`),
			Entry("string with quote", `a"b`, `"a\"b"`),
			Entry("bool true", true, "true"),
			Entry("bool false", false, "false"),
			Entry("int", 42, "42"),
			Entry("zero", 0, "0"),
		)
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
			Entry("slash", `a/b`, "a%2Fb"),
			Entry("plus", `a+b`, "a%2Bb"),
			Entry("html", `<script>`, "%3Cscript%3E"),
			Entry("injection", `name";alert(1)`, "name%22%3Balert%281%29"),
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

	Describe("RenovatorOpenXData", func() {
		It("builds an Alpine expression with a JSON-escaped key", func() {
			Expect(RenovatorOpenXData("simple")).
				To(Equal(`{ open: $persist(false).as("renovator-simple") }`))
		})

		It("escapes injection attempts in the name", func() {
			got := RenovatorOpenXData(`evil"injection`)
			Expect(got).NotTo(ContainSubstring(`evil"injection`))
			Expect(got).To(ContainSubstring(`\"`))
		})
	})

	Describe("JobListXData", func() {
		It("builds an Alpine expression that references jobList by composite key", func() {
			Expect(JobListXData("ns", "name")).To(Equal(`jobList("ns-name")`))
		})
	})

	Describe("LogViewerXData", func() {
		It("builds an Alpine expression with all four arguments", func() {
			Expect(LogViewerXData("ns", "runner", "job", true)).
				To(Equal(`logViewer("ns", "runner", "job", true)`))
		})

		It("escapes injection attempts in the arguments", func() {
			got := LogViewerXData(`n"s`, `r'`, `j\`, false)
			Expect(got).NotTo(ContainSubstring(`n"s`))
		})
	})

	Describe("SelectJobExpr", func() {
		It("builds an Alpine expression that selects a job and reads the URL from hx-get", func() {
			Expect(SelectJobExpr("name")).
				To(Equal(`selectJob($event.currentTarget.getAttribute('hx-get'), "name")`))
		})
	})
})

var _ = Describe("sanitize regression cases", func() {
	// Lock-in: ensure none of the helpers reintroduce unsafe string interpolation
	// by silently passing through characters that would break out of a JS string
	// or a URL query.
	It("JSString output always round-trips through json.Unmarshal to the input", func() {
		inputs := []string{
			`"`, `a"b`, `"<script>`, `'`, `<`, `>`, `&`, "\n", "\t", "\u2028", `\`, "",
		}
		for _, in := range inputs {
			got := JSString(in)

			var decoded string
			Expect(json.Unmarshal([]byte(got), &decoded)).To(Succeed(),
				"JSString(%q) = %s is not a valid JSON string", in, got)
			Expect(decoded).To(Equal(in),
				"JSString(%q) = %s did not round-trip (got %q)", in, got, decoded)
		}
	})

	It("QueryEscape output is always safe to drop into a query string", func() {
		// Characters that, if emitted unescaped, would either change URL
		// structure or carry HTML/JS semantics. (`+` is the SAFE encoding of
		// space in application/x-www-form-urlencoded, so it is not dangerous.)
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
