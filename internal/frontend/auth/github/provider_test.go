package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func newTestProvider(endpoint string, httpClient *http.Client) *GitHubProvider {
	return &GitHubProvider{
		endpoint:   endpoint,
		httpClient: httpClient,
	}
}

func newTestClient() *http.Client {
	return &http.Client{}
}

func newTestClientWithToken(token string) *http.Client {
	return &http.Client{
		Transport: &tokenTransport{token: token},
	}
}

type tokenTransport struct {
	token string
	base  http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+t.token)

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	return base.RoundTrip(req2)
}

var _ = Describe("GitHubProvider", func() {
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
		It("returns all repos with write access from GitHub response across pages", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/api/v3/user/repos"))

				page := r.URL.Query().Get("page")

				w.Header().Set("Content-Type", "application/json")

				switch page {
				case "1":
					_, _ = fmt.Fprint(w, `[
						{"full_name":"owner/repo1","permissions":{"push":true}},
						{"full_name":"owner/repo2","permissions":{"push":true}}
					]`)
				default:
					_, _ = fmt.Fprint(w, `[]`)
				}
			}))

			provider := newTestProvider(server.URL, server.Client())

			repos, err := provider.GetUserRepos(ctx, newTestClient())
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveKeyWithValue("owner/repo1", true))
			Expect(repos).To(HaveKeyWithValue("owner/repo2", true))
			Expect(repos).To(HaveLen(2))
		})

		It("sets the Authorization header to Bearer <token>", func() {
			var gotAuth string

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `[]`)
			}))

			provider := newTestProvider(server.URL, server.Client())

			_, err := provider.GetUserRepos(ctx, newTestClientWithToken("my-secret-token"))
			Expect(err).NotTo(HaveOccurred())
			Expect(gotAuth).To(Equal("Bearer my-secret-token"))
		})

		It("returns an error for non-200 responses", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forbidden", http.StatusForbidden)
			}))

			provider := newTestProvider(server.URL, server.Client())

			_, err := provider.GetUserRepos(ctx, newTestClient())
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(errUnexpectedStatus))
		})

		It("returns error when context is already cancelled", func() {
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `[]`)
			}))

			provider := newTestProvider(server.URL, server.Client())

			repos, err := provider.GetUserRepos(cancelCtx, newTestClient())
			Expect(err).To(HaveOccurred())
			Expect(repos).To(BeEmpty())
		})

		It("filters out repos without write access", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				page := r.URL.Query().Get("page")
				if page == "1" {
					_, _ = fmt.Fprint(w, `[
						{"full_name":"owner/read-only","permissions":{"push":false}},
						{"full_name":"owner/write-access","permissions":{"push":true}}
					]`)
				} else {
					_, _ = fmt.Fprint(w, `[]`)
				}
			}))

			provider := newTestProvider(server.URL, server.Client())

			repos, err := provider.GetUserRepos(ctx, newTestClient())
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveKeyWithValue("owner/write-access", true))
			Expect(repos).NotTo(HaveKey("owner/read-only"))
			Expect(repos).To(HaveLen(1))
		})

		It("returns an error for 401 response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			}))

			provider := newTestProvider(server.URL, server.Client())

			repos, err := provider.GetUserRepos(ctx, newTestClient())
			Expect(err).To(HaveOccurred())
			Expect(repos).To(BeEmpty())
		})
	})

	Describe("IsUserRepo", func() {
		It("returns true for 200 response with push access", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/api/v3/repos/owner/repo1"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{
					"full_name": "owner/repo1",
					"permissions": {"push": true}
				}`)
			}))

			provider := newTestProvider(server.URL, server.Client())

			accessible, err := provider.IsUserRepo(ctx, newTestClient(), "owner/repo1")
			Expect(err).NotTo(HaveOccurred())
			Expect(accessible).To(BeTrue())
		})

		It("returns false for 200 response without push access", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/api/v3/repos/owner/repo1"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{
					"full_name": "owner/repo1",
					"permissions": {"push": false}
				}`)
			}))

			provider := newTestProvider(server.URL, server.Client())

			accessible, err := provider.IsUserRepo(ctx, newTestClient(), "owner/repo1")
			Expect(err).NotTo(HaveOccurred())
			Expect(accessible).To(BeFalse())
		})

		It("returns false for 404 response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			}))

			provider := newTestProvider(server.URL, server.Client())

			accessible, err := provider.IsUserRepo(ctx, newTestClient(), "owner/missing")
			Expect(err).NotTo(HaveOccurred())
			Expect(accessible).To(BeFalse())
		})

		It("returns false for 403 response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forbidden", http.StatusForbidden)
			}))

			provider := newTestProvider(server.URL, server.Client())

			accessible, err := provider.IsUserRepo(ctx, newTestClient(), "owner/private")
			Expect(err).NotTo(HaveOccurred())
			Expect(accessible).To(BeFalse())
		})

		It("returns error for unexpected status", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "error", http.StatusInternalServerError)
			}))

			provider := newTestProvider(server.URL, server.Client())

			_, err := provider.IsUserRepo(ctx, newTestClient(), "owner/repo1")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(errUnexpectedStatus))
		})

		It("sets the Authorization header", func() {
			var gotAuth string

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{
					"full_name": "owner/repo1",
					"permissions": {"push": true}
				}`)
			}))

			provider := newTestProvider(server.URL, server.Client())

			_, err := provider.IsUserRepo(ctx, newTestClientWithToken("my-token"), "owner/repo1")
			Expect(err).NotTo(HaveOccurred())
			Expect(gotAuth).To(Equal("Bearer my-token"))
		})

		It("returns an error for 401 response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			}))

			provider := newTestProvider(server.URL, server.Client())

			accessible, err := provider.IsUserRepo(ctx, newTestClient(), "owner/repo1")
			Expect(err).To(HaveOccurred())
			Expect(accessible).To(BeFalse())
		})
	})
})
