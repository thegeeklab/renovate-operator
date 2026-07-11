package gitea

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thegeeklab/renovate-operator/internal/provider"
)

var _ = Describe("Gitea Provider", func() {
	Describe("parseRepoName", func() {
		It("should successfully parse a valid repository name", func() {
			owner, repo, err := parseRepoName("thegeeklab/renovate-operator")
			Expect(err).NotTo(HaveOccurred())
			Expect(owner).To(Equal("thegeeklab"))
			Expect(repo).To(Equal("renovate-operator"))
		})

		It("should return an error for an invalid repository name without slash", func() {
			_, _, err := parseRepoName("invalid-repo-name")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid repository name format"))
		})

		It("should return an error for an invalid repository name with too many slashes", func() {
			_, _, err := parseRepoName("thegeeklab/renovate-operator/extra")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid repository name format"))
		})
	})

	Describe("sanitizeEndpoint", func() {
		It("should leave clean URLs unchanged", func() {
			Expect(sanitizeEndpoint("https://gitea.example.com")).To(Equal("https://gitea.example.com"))
		})

		It("should strip trailing slashes", func() {
			Expect(sanitizeEndpoint("https://gitea.example.com/")).To(Equal("https://gitea.example.com"))
		})

		It("should strip the /api/v1 suffix", func() {
			Expect(sanitizeEndpoint("https://gitea.example.com/api/v1")).To(Equal("https://gitea.example.com"))
		})

		It("should strip both the /api/v1 suffix and trailing slash", func() {
			Expect(sanitizeEndpoint("https://gitea.example.com/api/v1/")).To(Equal("https://gitea.example.com"))
		})
	})

	Context("API Interactions", func() {
		var (
			ctx        context.Context
			mockServer *httptest.Server
			mux        *http.ServeMux
			p          *Provider
		)

		BeforeEach(func() {
			ctx = context.Background()
			mux = http.NewServeMux()
			mockServer = httptest.NewServer(mux)

			mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"version": "1.19.0"}`))
			})

			var err error

			p, err = NewProvider(ctx, mockServer.URL, "dummy-token")
			Expect(err).NotTo(HaveOccurred())
			Expect(p).NotTo(BeNil())
			Expect(p.client).NotTo(BeNil())
		})

		AfterEach(func() {
			mockServer.Close()
		})

		Describe("NewProvider", func() {
			It("should successfully create a new provider and ping the version endpoint", func() {
				Expect(p.client).NotTo(BeNil())
			})
		})

		Describe("EnsureWebhook", func() {
			It("should fail if the user lacks admin permissions on the repository", func() {
				mux.HandleFunc(
					"/api/v1/repos/thegeeklab/renovate-operator",
					func(w http.ResponseWriter, r *http.Request) {
						Expect(r.Method).To(Equal(http.MethodGet))
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"permissions": {"admin": false}}`))
					},
				)

				_, err := p.EnsureWebhook(ctx, "thegeeklab/renovate-operator", "https://hook.url", "dummy-secret")
				Expect(err).To(MatchError(errMissingAdmin))
			})

			It("should update and return the existing webhook ID if the URL already matches", func() {
				mux.HandleFunc(
					"/api/v1/repos/thegeeklab/renovate-operator",
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"permissions": {"admin": true}}`))
					},
				)

				mux.HandleFunc(
					"/api/v1/repos/thegeeklab/renovate-operator/hooks",
					func(w http.ResponseWriter, r *http.Request) {
						Expect(r.Method).To(Equal(http.MethodGet))
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`[{"id": 123, "config": {"url": "https://hook.url"}}]`))
					},
				)

				mux.HandleFunc(
					"/api/v1/repos/thegeeklab/renovate-operator/hooks/123",
					func(w http.ResponseWriter, r *http.Request) {
						Expect(r.Method).To(Equal(http.MethodPatch))
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"id": 123, "config": {"url": "https://hook.url"}}`))
					},
				)

				id, err := p.EnsureWebhook(ctx, "thegeeklab/renovate-operator", "https://hook.url", "dummy-secret")
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal("123"))
			})

			It("should create a new webhook if no matching URL is found", func() {
				mux.HandleFunc(
					"/api/v1/repos/thegeeklab/renovate-operator",
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"permissions": {"admin": true}}`))
					},
				)

				mux.HandleFunc(
					"/api/v1/repos/thegeeklab/renovate-operator/hooks",
					func(w http.ResponseWriter, r *http.Request) {
						if r.Method == http.MethodGet {
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(`[]`))

							return
						}

						if r.Method == http.MethodPost {
							w.WriteHeader(http.StatusCreated)
							_, _ = w.Write([]byte(`{"id": 999}`))

							return
						}

						w.WriteHeader(http.StatusMethodNotAllowed)
					},
				)

				id, err := p.EnsureWebhook(ctx, "thegeeklab/renovate-operator", "https://new.hook.url", "dummy-secret")
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal("999"))
			})
		})

		Describe("DeleteWebhook", func() {
			It("should return early without error if the webhook ID is empty", func() {
				err := p.DeleteWebhook(ctx, "thegeeklab/renovate-operator", "")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should successfully delete the webhook from the remote", func() {
				mux.HandleFunc(
					"/api/v1/repos/thegeeklab/renovate-operator/hooks/123",
					func(w http.ResponseWriter, r *http.Request) {
						Expect(r.Method).To(Equal(http.MethodDelete))
						w.WriteHeader(http.StatusNoContent)
					},
				)

				err := p.DeleteWebhook(ctx, "thegeeklab/renovate-operator", "123")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an error if the webhook ID is not a valid integer", func() {
				err := p.DeleteWebhook(ctx, "thegeeklab/renovate-operator", "invalid-id")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid webhook ID format"))
			})
		})

		Describe("ListRepos", func() {
			It("should filter repositories by topics", func() {
				mux.HandleFunc("/api/v1/user/repos", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[
						{"id": 1, "name": "repo1", "full_name": "owner/repo1", "topics": ["renovate", "production"]},
						{"id": 2, "name": "repo2", "full_name": "owner/repo2", "topics": ["renovate"]},
						{"id": 3, "name": "repo3", "full_name": "owner/repo3", "topics": ["production"]}
					]`))
				})

				repos, err := p.ListRepos(ctx, provider.ListReposOptions{
					Topics: []string{"renovate", "production"},
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(repos).To(HaveLen(1))
				Expect(repos[0].Name).To(Equal("owner/repo1"))
			})

			It("should return all repositories when no topics filter is specified", func() {
				mux.HandleFunc("/api/v1/user/repos", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[
						{"id": 1, "name": "repo1", "full_name": "owner/repo1", "topics": ["renovate"]},
						{"id": 2, "name": "repo2", "full_name": "owner/repo2", "topics": []}
					]`))
				})

				repos, err := p.ListRepos(ctx, provider.ListReposOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(repos).To(HaveLen(2))
			})
		})
	})
})
