package gitea

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
)

func newTestProvider(forgeURL string, httpClient *http.Client) *GiteaProvider {
	return &GiteaProvider{
		forgeURL:   forgeURL,
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
		It("returns all repos with write access from Gitea response across pages", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/api/v1/user/repos"))

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

		It("sets the Authorization header to token <token>", func() {
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
				Expect(r.URL.Path).To(Equal("/api/v1/repos/owner/repo1"))
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
				Expect(r.URL.Path).To(Equal("/api/v1/repos/owner/repo1"))
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

	Describe("RefreshToken", func() {
		It("returns error when refresh token is empty", func() {
			provider := newTestProvider("http://example.com", &http.Client{})

			user, err := provider.RefreshToken(ctx, "")
			Expect(err).To(MatchError(errNoRefreshToken))
			Expect(user).To(BeNil())
		})
	})

	Describe("HandleCallback", func() {
		var (
			privKey *rsa.PrivateKey
			oidcSrv *oidctest.Server
			testSrv *httptest.Server
		)

		BeforeEach(func() {
			var err error

			privKey, err = rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).NotTo(HaveOccurred())

			oidcSrv = &oidctest.Server{
				PublicKeys: []oidctest.PublicKey{
					{
						PublicKey: privKey.Public(),
						KeyID:     "test-key-id",
						Algorithm: oidc.RS256,
					},
				},
			}

			testSrv = httptest.NewServer(oidcSrv)
			oidcSrv.SetIssuer(testSrv.URL)
		})

		AfterEach(func() {
			if testSrv != nil {
				testSrv.Close()
			}
		})

		It("successfully exchanges code and extracts user info", func() {
			tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					claims := fmt.Sprintf(`{
						"iss": "%s",
						"aud": "test-client",
						"sub": "user-123",
						"email": "test@example.com",
						"name": "Test User",
						"exp": %d
					}`, testSrv.URL, time.Now().Add(time.Hour).Unix())

					idToken := oidctest.SignIDToken(privKey, "test-key-id", oidc.RS256, claims)

					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, `{
						"access_token": "test-access-token",
						"token_type": "Bearer",
						"refresh_token": "test-refresh-token",
						"expires_in": 3600,
						"id_token": "%s"
					}`, idToken)

					return
				}

				http.NotFound(w, r)
			}))
			defer tokenSrv.Close()

			provider := &GiteaProvider{
				name:         "test",
				issuerURL:    testSrv.URL,
				clientID:     "test-client",
				clientSecret: "test-secret",
				redirectURL:  "http://localhost/callback",
				httpClient:   testSrv.Client(),
				oauth2Config: &oauth2.Config{
					ClientID:     "test-client",
					ClientSecret: "test-secret",
					RedirectURL:  "http://localhost/callback",
					Endpoint: oauth2.Endpoint{
						AuthURL:  testSrv.URL + "/auth",
						TokenURL: tokenSrv.URL + "/token",
					},
					Scopes: []string{oidc.ScopeOpenID, "profile", "email"},
				},
			}

			providerCtx := oidc.ClientContext(ctx, testSrv.Client())
			oidcProvider, err := oidc.NewProvider(providerCtx, testSrv.URL)
			Expect(err).NotTo(HaveOccurred())

			provider.verifier = oidcProvider.Verifier(&oidc.Config{ClientID: "test-client"})

			user, err := provider.HandleCallback(ctx, "test-code")
			Expect(err).NotTo(HaveOccurred())
			Expect(user).NotTo(BeNil())
			Expect(user.Email).To(Equal("test@example.com"))
			Expect(user.Name).To(Equal("Test User"))
			Expect(user.Subject).To(Equal("user-123"))
			Expect(user.AccessToken).To(Equal("test-access-token"))
			Expect(user.RefreshToken).To(Equal("test-refresh-token"))
		})
	})
})
