package gitea

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GiteaAccessChecker", func() {
	var (
		server *httptest.Server
		ctx    context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	Describe("GetAccessibleRepos", func() {
		It("returns all repos from wrapped Gitea response across pages", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/api/v1/repos/search"))

				page := r.URL.Query().Get("page")

				w.Header().Set("Content-Type", "application/json")

				switch page {
				case "1":
					_, _ = fmt.Fprint(w, `{"ok":true,"data":[{"full_name":"owner/repo1"},{"full_name":"owner/repo2"}]}`)
				default:
					_, _ = fmt.Fprint(w, `{"ok":true,"data":[]}`)
				}
			}))

			checker := NewGiteaAccessChecker(server.URL, "test-token", server.Client())

			repos, err := checker.GetAccessibleRepos(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveKeyWithValue("owner/repo1", true))
			Expect(repos).To(HaveKeyWithValue("owner/repo2", true))
			Expect(repos).To(HaveLen(2))
		})

		It("sets the Authorization header to token <token>", func() {
			var gotAuth string

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"ok":true,"data":[]}`)
			}))

			checker := NewGiteaAccessChecker(server.URL, "my-secret-token", server.Client())

			_, err := checker.GetAccessibleRepos(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(gotAuth).To(Equal("token my-secret-token"))
		})

		It("returns an error for non-200 responses", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forbidden", http.StatusForbidden)
			}))

			checker := NewGiteaAccessChecker(server.URL, "test-token", server.Client())

			_, err := checker.GetAccessibleRepos(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(errUnexpectedStatus))
		})
	})
})
