package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thegeeklab/renovate-operator/internal/provider"
)

var _ = Describe("GitLab Provider", func() {
	DescribeTable(
		"normalizes endpoints",
		func(endpoint, expectedAPI, expectedForge string) {
			apiURL, forgeURL := normalizeEndpoint(endpoint)
			Expect(apiURL).To(Equal(expectedAPI))
			Expect(forgeURL).To(Equal(expectedForge))
		},
		Entry("empty endpoint", "", "https://gitlab.com/api/v4/", "https://gitlab.com"),
		Entry("GitLab.com web endpoint", "https://gitlab.com/", "https://gitlab.com/api/v4/", "https://gitlab.com"),
		Entry("GitLab.com API endpoint", "https://gitlab.com/api/v4/", "https://gitlab.com/api/v4/", "https://gitlab.com"),
		Entry(
			"self-managed web endpoint",
			"https://gitlab.example.com/",
			"https://gitlab.example.com/api/v4/",
			"https://gitlab.example.com",
		),
		Entry(
			"self-managed API endpoint",
			"https://gitlab.example.com/api/v4/",
			"https://gitlab.example.com/api/v4/",
			"https://gitlab.example.com",
		),
	)

	DescribeTable(
		"validates project paths",
		func(projectPath string, valid bool) {
			parsed, err := parseProjectPath(projectPath)
			if valid {
				Expect(err).NotTo(HaveOccurred())
				Expect(parsed).To(Equal(strings.TrimSpace(projectPath)))

				return
			}

			Expect(err).To(MatchError(ContainSubstring("invalid project name format")))
		},
		Entry("top-level group", "group/project", true),
		Entry("nested groups", "group/subgroup/deeper/project", true),
		Entry("blank", "", false),
		Entry("missing namespace", "project", false),
		Entry("empty middle segment", "group//project", false),
		Entry("empty first segment", "/group/project", false),
		Entry("empty last segment", "group/project/", false),
	)

	Context("API interactions", func() {
		var (
			ctx      context.Context
			server   *httptest.Server
			requests chan *http.Request
			p        *Provider
			handler  http.HandlerFunc
		)

		BeforeEach(func() {
			ctx = context.Background()
			requests = make(chan *http.Request, 20)
			handler = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.Clone(r.Context())

				handler(w, r)
			}))

			var err error

			p, err = NewProvider(ctx, server.URL, "test-token")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			server.Close()
		})

		It("gets the authenticated identity", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/api/v4/user"))
				Expect(r.Header.Get("PRIVATE-TOKEN")).To(Equal("test-token"))

				_, _ = w.Write([]byte(`{"id":1,"username":"renovate-bot"}`))
			}

			identity, err := p.GetIdentity()
			Expect(err).NotTo(HaveOccurred())
			Expect(identity).To(Equal("renovate-bot"))
		})

		It("resolves nested project paths as one escaped identifier", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.EscapedPath()).To(Equal("/api/v4/projects/group%2Fsubgroup%2Fproject"))

				_, _ = w.Write([]byte(`{"web_url":"https://gitlab.example/group/subgroup/project"}`))
			}

			repoURL, err := p.RepoURL(ctx, "group/subgroup/project")
			Expect(err).NotTo(HaveOccurred())
			Expect(repoURL).To(Equal("https://gitlab.example/group/subgroup/project"))
		})

		It("does not double escape project identifiers", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.RequestURI).NotTo(ContainSubstring("%252F"))
				Expect(r.URL.EscapedPath()).To(Equal("/api/v4/projects/group%2Fproject"))

				_, _ = w.Write([]byte(`{}`))
			}

			repoURL, err := p.RepoURL(ctx, "group/project")
			Expect(err).NotTo(HaveOccurred())
			Expect(repoURL).To(Equal(server.URL + "/group/project"))
		})

		It("wraps project lookup errors", func() {
			_, err := p.RepoURL(ctx, "group/project")
			Expect(err).To(MatchError(ContainSubstring("failed to fetch project")))
		})

		It("paginates and filters projects while preserving order", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/api/v4/projects"))
				query := r.URL.Query()
				Expect(query.Get("membership")).To(Equal("true"))
				Expect(query.Get("min_access_level")).To(Equal("30"))
				Expect(query.Get("with_merge_requests_enabled")).To(Equal("true"))
				Expect(query.Get("per_page")).To(Equal("50"))

				if query.Get("page") == "1" {
					w.Header().Set("X-Next-Page", "2")
					_, _ = w.Write([]byte(`[
						{"path_with_namespace":"group/first","topics":["renovate","prod"]},
						{"path_with_namespace":"group/fork","topics":["renovate","prod"],"forked_from_project":{"id":1}},
						{"path_with_namespace":"group/wrong-topic","topics":["renovate"]}
					]`))

					return
				}

				_, _ = w.Write([]byte(`[
					{"path_with_namespace":"group/subgroup/second","topics":["prod","renovate"]},
					{"path_with_namespace":"","topics":["prod","renovate"]}
				]`))
			}

			repos, err := p.ListRepos(ctx, provider.ListReposOptions{
				SkipForks: true,
				Topics:    []string{"renovate", "prod"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(Equal([]provider.Repo{
				{Name: "group/first", IsFork: false},
				{Name: "group/subgroup/second", IsFork: false},
			}))
		})

		It("reports project-list failures", func() {
			_, err := p.ListRepos(ctx, provider.ListReposOptions{})
			Expect(err).To(MatchError(ContainSubstring("failed to list projects")))
		})

		It("excludes projects marked for deletion when skipPendingDeletion is enabled", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/api/v4/projects"))

				_, _ = w.Write([]byte(`[
					{"path_with_namespace":"group/active","topics":[]},
					{"path_with_namespace":"group/pending-delete","topics":[],"marked_for_deletion_on":"2025-06-01T00:00:00.000Z"},
					{"path_with_namespace":"group/another-active","topics":[]}
				]`))
			}

			repos, err := p.ListRepos(ctx, provider.ListReposOptions{
				SkipPendingDeletion: true,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(Equal([]provider.Repo{
				{Name: "group/active", IsFork: false},
				{Name: "group/another-active", IsFork: false},
			}))
		})

		It("rejects hook management without Maintainer access", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"permissions":{"project_access":{"access_level":30}}}`))
			}

			_, err := p.EnsureWebhook(ctx, "group/project", "https://operator.example/hook", "secret")
			Expect(err).To(MatchError(errMissingMaintainer))
		})

		It("creates a project hook using inherited Maintainer access", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/hooks"):
					_, _ = w.Write([]byte(`[]`))
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/hooks"):
					assertHookPayload(r, "https://operator.example/hook", "new-secret")
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"id":99}`))
				default:
					_, _ = w.Write([]byte(`{"permissions":{"group_access":{"access_level":40}}}`))
				}
			}

			hookID, err := p.EnsureWebhook(ctx, "group/subgroup/project", "https://operator.example/hook", "new-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(hookID).To(Equal("99"))
		})

		It("finds and updates a project hook on a later page", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/hooks"):
					if r.URL.Query().Get("page") == "1" {
						w.Header().Set("X-Next-Page", "2")
						_, _ = w.Write([]byte(`[{"id":1,"url":"https://other.example/hook"}]`))

						return
					}

					_, _ = w.Write([]byte(`[{"id":42,"url":"https://operator.example/hook"}]`))
				case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/hooks/42"):
					assertHookPayload(r, "https://operator.example/hook", "rotated-secret")

					_, _ = w.Write([]byte(`{"id":42}`))
				default:
					_, _ = w.Write([]byte(`{"permissions":{"project_access":{"access_level":40}}}`))
				}
			}

			hookID, err := p.EnsureWebhook(ctx, "group/project", "https://operator.example/hook", "rotated-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(hookID).To(Equal("42"))
		})

		DescribeTable(
			"validates hook IDs",
			func(hookID string, wantError bool) {
				err := p.DeleteWebhook(ctx, "group/project", hookID)
				if wantError {
					Expect(err).To(MatchError(ContainSubstring("invalid webhook ID format")))

					return
				}

				Expect(err).NotTo(HaveOccurred())
			},
			Entry("empty ID", "", false),
			Entry("non-numeric ID", "invalid", true),
		)

		It("deletes a project hook", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodDelete))
				Expect(r.URL.EscapedPath()).To(Equal("/api/v4/projects/group%2Fproject/hooks/7"))
				w.WriteHeader(http.StatusNoContent)
			}

			Expect(p.DeleteWebhook(ctx, "group/project", "7")).To(Succeed())
		})

		It("treats a missing project hook as deleted", func() {
			Expect(p.DeleteWebhook(ctx, "group/project", "7")).To(Succeed())
		})

		It("wraps non-404 project-hook deletion errors", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"failed"}`))
			}

			err := p.DeleteWebhook(ctx, "group/project", "7")
			Expect(err).To(MatchError(ContainSubstring("failed to delete project hook 7")))
		})
	})
})

func assertHookPayload(r *http.Request, webhookURL, secret string) {
	Expect(r.Header.Get("Content-Type")).To(ContainSubstring("application/json"))

	payload := map[string]any{}
	Expect(json.NewDecoder(r.Body).Decode(&payload)).To(Succeed())
	Expect(payload).To(HaveKeyWithValue("url", webhookURL))
	Expect(payload).To(HaveKeyWithValue("token", secret))
	Expect(payload).To(HaveKeyWithValue("push_events", true))
	Expect(payload).To(HaveKeyWithValue("merge_requests_events", true))
	Expect(payload).To(HaveKeyWithValue("issues_events", true))
	Expect(payload).To(HaveKeyWithValue("enable_ssl_verification", true))
}
