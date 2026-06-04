package gitea

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func newTestProvider(forgeURL string, httpClient *http.Client) *GiteaProvider {
	return &GiteaProvider{
		forgeURL:   forgeURL,
		httpClient: httpClient,
	}
}

var _ = Describe("GiteaProvider", func() {
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

	Describe("GetUserRepos", func() {
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

			provider := newTestProvider(server.URL, server.Client())

			repos, err := provider.GetUserRepos(ctx, "test-token")
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

			provider := newTestProvider(server.URL, server.Client())

			_, err := provider.GetUserRepos(ctx, "my-secret-token")
			Expect(err).NotTo(HaveOccurred())
			Expect(gotAuth).To(Equal("token my-secret-token"))
		})

		It("returns an error for non-200 responses", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forbidden", http.StatusForbidden)
			}))

			provider := newTestProvider(server.URL, server.Client())

			_, err := provider.GetUserRepos(ctx, "test-token")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(errUnexpectedStatus))
		})

		It("returns error when context is already cancelled", func() {
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"ok":true,"data":[]}`)
			}))

			provider := newTestProvider(server.URL, server.Client())

			repos, err := provider.GetUserRepos(cancelCtx, "test-token")
			Expect(err).To(HaveOccurred())
			Expect(repos).To(BeEmpty())
		})
	})

	Describe("IsUserRepo", func() {
		It("returns true for 200 response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/api/v1/repos/owner/repo1"))
				w.WriteHeader(http.StatusOK)
			}))

			provider := newTestProvider(server.URL, server.Client())

			accessible, err := provider.IsUserRepo(ctx, "test-token", "owner/repo1")
			Expect(err).NotTo(HaveOccurred())
			Expect(accessible).To(BeTrue())
		})

		It("returns false for 404 response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			}))

			provider := newTestProvider(server.URL, server.Client())

			accessible, err := provider.IsUserRepo(ctx, "test-token", "owner/missing")
			Expect(err).NotTo(HaveOccurred())
			Expect(accessible).To(BeFalse())
		})

		It("returns false for 403 response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forbidden", http.StatusForbidden)
			}))

			provider := newTestProvider(server.URL, server.Client())

			accessible, err := provider.IsUserRepo(ctx, "test-token", "owner/private")
			Expect(err).NotTo(HaveOccurred())
			Expect(accessible).To(BeFalse())
		})

		It("returns error for unexpected status", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "error", http.StatusInternalServerError)
			}))

			provider := newTestProvider(server.URL, server.Client())

			_, err := provider.IsUserRepo(ctx, "test-token", "owner/repo1")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(errUnexpectedStatus))
		})

		It("sets the Authorization header", func() {
			var gotAuth string

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")

				w.WriteHeader(http.StatusOK)
			}))

			provider := newTestProvider(server.URL, server.Client())

			_, err := provider.IsUserRepo(ctx, "my-token", "owner/repo1")
			Expect(err).NotTo(HaveOccurred())
			Expect(gotAuth).To(Equal("token my-token"))
		})
	})

	Describe("parseRetryAfter", func() {
		It("parses seconds format", func() {
			resp := &http.Response{Header: http.Header{}}
			resp.Header.Set("Retry-After", "30")

			provider := newTestProvider("http://example.com", &http.Client{})

			duration := provider.parseRetryAfter(resp)
			Expect(duration).To(Equal(30 * time.Second))
		})

		It("parses HTTP date format", func() {
			futureTime := time.Now().Add(60 * time.Second)
			resp := &http.Response{Header: http.Header{}}
			resp.Header.Set("Retry-After", futureTime.UTC().Format(http.TimeFormat))

			provider := newTestProvider("http://example.com", &http.Client{})

			duration := provider.parseRetryAfter(resp)
			Expect(duration).To(BeNumerically("~", 60*time.Second, 2*time.Second))
		})

		It("returns 0 for empty header", func() {
			resp := &http.Response{Header: http.Header{}}

			provider := newTestProvider("http://example.com", &http.Client{})

			duration := provider.parseRetryAfter(resp)
			Expect(duration).To(BeZero())
		})

		It("returns 0 for invalid format", func() {
			resp := &http.Response{Header: http.Header{}}
			resp.Header.Set("Retry-After", "not-a-valid-value")

			provider := newTestProvider("http://example.com", &http.Client{})

			duration := provider.parseRetryAfter(resp)
			Expect(duration).To(BeZero())
		})
	})
})
