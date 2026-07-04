package github

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GitHub Provider", func() {
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
		It("should convert github.com to api.github.com", func() {
			Expect(sanitizeEndpoint("https://github.com")).To(Equal("https://api.github.com"))
		})

		It("should strip trailing slashes", func() {
			Expect(sanitizeEndpoint("https://github.com/")).To(Equal("https://api.github.com"))
		})

		It("should append /api/v3 to custom endpoints", func() {
			Expect(sanitizeEndpoint("https://github.example.com")).To(Equal("https://github.example.com/api/v3"))
		})

		It("should strip trailing slashes from custom endpoints and append /api/v3", func() {
			Expect(sanitizeEndpoint("https://github.example.com/")).To(Equal("https://github.example.com/api/v3"))
		})

		It("should not duplicate /api/v3 if already present", func() {
			Expect(sanitizeEndpoint("https://github.example.com/api/v3")).To(Equal("https://github.example.com/api/v3"))
		})
	})

	Describe("deriveForgeURL", func() {
		It("should return github.com for empty endpoint", func() {
			Expect(deriveForgeURL("")).To(Equal("https://github.com"))
		})

		It("should return github.com for github.com endpoint", func() {
			Expect(deriveForgeURL("https://github.com")).To(Equal("https://github.com"))
		})

		It("should return github.com for api.github.com endpoint", func() {
			Expect(deriveForgeURL("https://api.github.com")).To(Equal("https://github.com"))
		})

		It("should strip /api/v3 from enterprise endpoints", func() {
			Expect(deriveForgeURL("https://github.example.com/api/v3")).To(Equal("https://github.example.com"))
		})

		It("should return enterprise URL unchanged if no /api/v3 suffix", func() {
			Expect(deriveForgeURL("https://github.example.com")).To(Equal("https://github.example.com"))
		})
	})

	Context("API Interactions", func() {
		var (
			ctx        context.Context
			mockServer *httptest.Server
			mux        *http.ServeMux
			provider   *Provider
		)

		BeforeEach(func() {
			ctx = context.Background()
			mux = http.NewServeMux()
			mockServer = httptest.NewServer(mux)

			var err error

			provider, err = NewProvider(ctx, mockServer.URL, "dummy-token")
			Expect(err).NotTo(HaveOccurred())
			Expect(provider).NotTo(BeNil())
		})

		AfterEach(func() {
			mockServer.Close()
		})

		Describe("NewProvider", func() {
			It("should successfully create a new provider", func() {
				Expect(provider.client).NotTo(BeNil())
			})
		})

		Describe("GetIdentity", func() {
			It("should return the username of the authenticated user", func() {
				mux.HandleFunc("/api/v3/user", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Method).To(Equal(http.MethodGet))
					Expect(r.Header.Get("Authorization")).To(ContainSubstring("Bearer"))
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"login": "testuser", "id": 123}`))
				})

				username, err := provider.GetIdentity()
				Expect(err).NotTo(HaveOccurred())
				Expect(username).To(Equal("testuser"))
			})
		})

		Describe("EnsureWebhook", func() {
			It("should fail if the user lacks admin permissions on the repository", func() {
				mux.HandleFunc("/api/v3/repos/thegeeklab/renovate-operator", func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Method).To(Equal(http.MethodGet))
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"permissions": {"admin": false}}`))
				})

				_, err := provider.EnsureWebhook(ctx, "thegeeklab/renovate-operator", "https://hook.url", "dummy-secret")
				Expect(err).To(MatchError(errMissingAdmin))
			})

			It("should create a new webhook if no matching URL is found", func() {
				mux.HandleFunc("/api/v3/repos/thegeeklab/renovate-operator", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"permissions": {"admin": true}}`))
				})

				mux.HandleFunc("/api/v3/repos/thegeeklab/renovate-operator/hooks", func(w http.ResponseWriter, r *http.Request) {
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
				})

				id, err := provider.EnsureWebhook(ctx, "thegeeklab/renovate-operator", "https://new.hook.url", "dummy-secret")
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal("999"))
			})
		})

		Describe("DeleteWebhook", func() {
			It("should return early without error if the webhook ID is empty", func() {
				err := provider.DeleteWebhook(ctx, "thegeeklab/renovate-operator", "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should successfully delete the webhook from the remote", func() {
				mux.HandleFunc(
					"/api/v3/repos/thegeeklab/renovate-operator/hooks/123",
					func(w http.ResponseWriter, r *http.Request) {
						Expect(r.Method).To(Equal(http.MethodDelete))
						w.WriteHeader(http.StatusNoContent)
					},
				)

				err := provider.DeleteWebhook(ctx, "thegeeklab/renovate-operator", "123")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error if the webhook ID is not a valid integer", func() {
				err := provider.DeleteWebhook(ctx, "thegeeklab/renovate-operator", "invalid-id")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid webhook ID format"))
			})
		})
	})
})
