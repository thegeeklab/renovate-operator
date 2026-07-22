package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thegeeklab/renovate-operator/internal/frontend/auth"
)

var _ = Describe("GitLabProvider", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable(
		"derives web and API URLs",
		func(endpoint, forgeURL, expectedWeb, expectedAPI string) {
			webURL, apiURL, err := deriveURLs(endpoint, forgeURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(webURL).To(Equal(expectedWeb))
			Expect(apiURL).To(Equal(expectedAPI))
		},
		Entry("GitLab.com default", "", "", defaultWebURL, defaultWebURL+"/api/v4"),
		Entry(
			"self-managed web endpoint",
			"https://gitlab.example.com/",
			"",
			"https://gitlab.example.com",
			"https://gitlab.example.com/api/v4",
		),
		Entry(
			"endpoint already contains API suffix",
			"https://gitlab.example.com/api/v4/",
			"",
			"https://gitlab.example.com",
			"https://gitlab.example.com/api/v4",
		),
		Entry(
			"explicit API forge URL",
			"https://gitlab.example.com",
			"https://api.example.com/custom/api/v4/",
			"https://gitlab.example.com",
			"https://api.example.com/custom/api/v4",
		),
	)

	It("configures OAuth endpoints, scopes, display, icon, and insecure TLS", func() {
		provider, err := NewGitLabProvider(ctx, auth.ProviderConfig{
			Name:         "work",
			Endpoint:     "https://gitlab.example.com",
			AuthURL:      "https://login.example.com/authorize",
			ClientID:     "client",
			ClientSecret: "secret",
			RedirectURL:  "https://operator.example/callback",
			Insecure:     true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(provider.Type()).To(Equal(auth.ProviderTypeGitLab))
		Expect(provider.DisplayName()).To(Equal("gitlab.example.com"))
		Expect(provider.IconURL()).To(Equal("https://gitlab.example.com/favicon.ico"))
		Expect(provider.oauth2Config.Endpoint.AuthURL).To(Equal("https://login.example.com/authorize"))
		Expect(provider.oauth2Config.Endpoint.TokenURL).To(Equal("https://gitlab.example.com/oauth/token"))
		Expect(provider.oauth2Config.Scopes).To(Equal([]string{"openid", "profile", "email", "read_api"}))

		transport, ok := provider.httpClient.Transport.(*http.Transport)
		Expect(ok).To(BeTrue())
		Expect(transport.TLSClientConfig.InsecureSkipVerify).To(BeTrue())
	})

	It("uses custom display and icon values", func() {
		provider, err := NewGitLabProvider(ctx, auth.ProviderConfig{
			Endpoint:    defaultWebURL,
			DisplayName: "Company GitLab",
			IconURL:     "https://example.com/gitlab.svg",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(provider.DisplayName()).To(Equal("Company GitLab"))
		Expect(provider.IconURL()).To(Equal("https://example.com/gitlab.svg"))
	})

	It("creates a login URL with state and all scopes", func() {
		provider, err := NewGitLabProvider(ctx, auth.ProviderConfig{
			Endpoint:    defaultWebURL,
			ClientID:    "client",
			RedirectURL: "https://operator.example/callback",
		})
		Expect(err).NotTo(HaveOccurred())

		loginURL := provider.LoginURL("state-token")
		Expect(loginURL).To(ContainSubstring("state=state-token"))
		Expect(loginURL).To(ContainSubstring("scope=openid+profile+email+read_api"))
	})

	It("exchanges a code and maps GitLab user fields", func() {
		server := gitlabOAuthServer()
		defer server.Close()

		provider, err := NewGitLabProvider(ctx, auth.ProviderConfig{
			Name:         "gitlab",
			Endpoint:     server.URL,
			ClientID:     "client",
			ClientSecret: "secret",
			RedirectURL:  "https://operator.example/callback",
		})
		Expect(err).NotTo(HaveOccurred())

		provider.httpClient = server.Client()

		user, err := provider.HandleCallback(ctx, "code")
		Expect(err).NotTo(HaveOccurred())
		Expect(user.Subject).To(Equal("123"))
		Expect(user.Name).To(Equal("Test User"))
		Expect(user.Email).To(Equal("test@example.com"))
		Expect(user.AccessToken).To(Equal("access-token"))
		Expect(user.RefreshToken).To(Equal("refresh-token"))
		Expect(user.Provider).To(Equal("gitlab"))
	})

	It("refreshes a session and maps hidden email from public_email", func() {
		server := gitlabOAuthServer()
		defer server.Close()

		provider, err := NewGitLabProvider(ctx, auth.ProviderConfig{Name: "gitlab", Endpoint: server.URL})
		Expect(err).NotTo(HaveOccurred())

		provider.httpClient = server.Client()

		user, err := provider.RefreshToken(ctx, "refresh-token")
		Expect(err).NotTo(HaveOccurred())
		Expect(user.Email).To(Equal("test@example.com"))
		Expect(user.AccessToken).To(Equal("access-token"))
	})

	It("rejects refresh without a token", func() {
		provider := &GitLabProvider{}
		user, err := provider.RefreshToken(ctx, "")
		Expect(err).To(MatchError(errNoRefreshToken))
		Expect(user).To(BeNil())
	})

	Describe("ValidateToken", func() {
		It("maps a valid PAT user and sends PRIVATE-TOKEN", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Header.Get("PRIVATE-TOKEN")).To(Equal("pat-token"))

				_, _ = fmt.Fprint(w, `{
					"id":123,"name":"Test User","email":"","public_email":"public@example.com",
					"avatar_url":"https://example.com/avatar.png"
				}`)
			}))
			defer server.Close()

			provider := &GitLabProvider{name: "gitlab", apiURL: server.URL, httpClient: server.Client()}
			user, err := provider.ValidateToken(ctx, "pat-token")
			Expect(err).NotTo(HaveOccurred())
			Expect(user.Email).To(Equal("public@example.com"))
			Expect(user.AccessToken).To(Equal("pat-token"))
		})

		It("rejects an empty PAT", func() {
			user, err := (&GitLabProvider{}).ValidateToken(ctx, "")
			Expect(err).To(MatchError(auth.ErrInvalidToken))
			Expect(user).To(BeNil())
		})

		It("wraps invalid PAT and malformed response failures", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			}))
			defer server.Close()

			provider := &GitLabProvider{apiURL: server.URL, httpClient: server.Client()}
			user, err := provider.ValidateToken(ctx, "invalid")
			Expect(err).To(MatchError(ContainSubstring("failed to validate token")))
			Expect(user).To(BeNil())
		})
	})

	Describe("GetUserRepos", func() {
		It("paginates with fixed constraints and preserves nested paths", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query()
				Expect(query.Get("membership")).To(Equal("true"))
				Expect(query.Get("min_access_level")).To(Equal("30"))
				Expect(query.Get("with_merge_requests_enabled")).To(Equal("true"))

				if query.Get("page") == "1" {
					for range defaultPageSize {
						_, _ = fmt.Fprint(w, "")
					}

					projects := make([]string, defaultPageSize)
					for i := range projects {
						projects[i] = fmt.Sprintf(`{"path_with_namespace":"group/repo-%d"}`, i)
					}

					_, _ = fmt.Fprintf(w, "[%s]", strings.Join(projects, ","))

					return
				}

				_, _ = fmt.Fprint(w, `[{"path_with_namespace":"group/subgroup/project"}]`)
			}))
			defer server.Close()

			provider := &GitLabProvider{apiURL: server.URL}
			repos, err := provider.GetUserRepos(ctx, server.Client())
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveLen(defaultPageSize + 1))
			Expect(repos).To(HaveKeyWithValue("group/subgroup/project", true))
		})

		It("returns partial results when a later page fails", func() {
			var requests atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("page") == "1" {
					projects := make([]string, defaultPageSize)
					for i := range projects {
						projects[i] = fmt.Sprintf(`{"path_with_namespace":"group/repo-%d"}`, i)
					}

					_, _ = fmt.Fprintf(w, "[%s]", strings.Join(projects, ","))

					return
				}

				requests.Add(1)
				w.WriteHeader(http.StatusForbidden)
			}))
			defer server.Close()

			provider := &GitLabProvider{apiURL: server.URL}
			repos, err := provider.GetUserRepos(ctx, server.Client())
			Expect(err).To(MatchError(ContainSubstring("partial results")))
			Expect(repos).To(HaveLen(defaultPageSize))
			Expect(requests.Load()).To(Equal(int32(1)))
		})

		It("retries server errors and honors retry-after parsing", func() {
			var requests atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) == 1 {
					w.WriteHeader(http.StatusInternalServerError)

					return
				}

				_, _ = fmt.Fprint(w, `[]`)
			}))
			defer server.Close()

			provider := &GitLabProvider{apiURL: server.URL}
			_, err := provider.GetUserRepos(ctx, server.Client())
			Expect(err).NotTo(HaveOccurred())
			Expect(requests.Load()).To(Equal(int32(2)))

			resp := &http.Response{Header: http.Header{"Retry-After": []string{"2"}}}
			Expect(parseRetryAfter(resp)).To(Equal(2 * time.Second))
		})

		It("returns an error for an already-cancelled context", func() {
			cancelled, cancel := context.WithCancel(ctx)
			cancel()

			repos, err := (&GitLabProvider{}).GetUserRepos(cancelled, &http.Client{})
			Expect(err).To(HaveOccurred())
			Expect(repos).To(BeEmpty())
		})
	})

	DescribeTable(
		"checks individual project access",
		func(status int, payload string, expected, expectError bool) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.EscapedPath()).To(Equal("/projects/group%2Fsubgroup%2Fproject"))
				w.WriteHeader(status)
				_, _ = fmt.Fprint(w, payload)
			}))
			defer server.Close()

			provider := &GitLabProvider{apiURL: server.URL}

			accessible, err := provider.IsUserRepo(ctx, server.Client(), "group/subgroup/project")
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(accessible).To(Equal(expected))
		},
		Entry("direct Developer", http.StatusOK, `{"permissions":{"project_access":{"access_level":30}}}`, true, false),
		Entry("inherited Maintainer", http.StatusOK, `{"permissions":{"group_access":{"access_level":40}}}`, true, false),
		Entry("Reporter", http.StatusOK, `{"permissions":{"project_access":{"access_level":20}}}`, false, false),
		Entry("forbidden", http.StatusForbidden, `{}`, false, false),
		Entry("missing", http.StatusNotFound, `{}`, false, false),
		Entry("server error", http.StatusInternalServerError, `{}`, false, true),
	)
})

func gitlabOAuthServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{
				"access_token":"access-token","token_type":"Bearer",
				"refresh_token":"refresh-token","expires_in":3600
			}`)
		case "/api/v4/user":
			Expect(r.Header.Get("Authorization")).To(Equal("Bearer access-token"))

			_, _ = fmt.Fprint(w, `{
				"id":123,"name":"Test User","email":"test@example.com",
				"public_email":"public@example.com","avatar_url":"https://example.com/avatar.png"
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
}
