package frontend

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gorilla/mux"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("APIHandler", func() {
	var (
		fakeClient  client.Client
		handler     *APIHandler
		scheme      *runtime.Scheme
		testObjects []runtime.Object
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		err := renovatev1beta1.AddToScheme(scheme)
		Expect(err).NotTo(HaveOccurred())

		testObjects = []runtime.Object{
			&renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "test-namespace",
				},
			},
			&renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "test-namespace",
				},
				Status: renovatev1beta1.GitRepoStatus{
					WebhookID: "12345",
				},
			},
			&renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "test-namespace",
				},
			},
			&renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "test-namespace",
				},
			},
		}

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(testObjects...).Build()
		handler = NewAPIHandler(fakeClient, nil)
	})

	Describe("NewAPIHandler", func() {
		It("should create a new APIHandler", func() {
			Expect(handler).NotTo(BeNil())
		})
	})

	Describe("RegisterRoutes", func() {
		It("should register API routes", func() {
			router := mux.NewRouter()

			handler.RegisterRoutes(router)

			testCases := []string{
				"/api/v1/version",
				"/api/v1/renovators",
				"/api/v1/gitrepos",
				"/api/v1/runners",
				"/api/v1/discoveries",
				"/api/v1/discovery/start",
				"/api/v1/discovery/status",
			}

			for _, path := range testCases {
				var found bool

				_ = router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
					routePath, err := route.GetPathTemplate()
					Expect(err).NotTo(HaveOccurred())

					if routePath == path {
						found = true
					}

					return nil
				})

				Expect(found).To(BeTrue(), "Route %s should be registered", path)
			}
		})
	})

	Describe("Endpoints", func() {
		Describe("getVersion", func() {
			It("should return version information", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
				w := httptest.NewRecorder()

				handler.getVersion(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))
				Expect(w.Body.String()).To(ContainSubstring(`"version":"v1.0.0"`))
			})
		})

		Describe("getRenovators", func() {
			It("should return renovators with sorting parameters", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/renovators?sort=name&order=desc", nil)
				w := httptest.NewRecorder()

				handler.getRenovators(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))
				Expect(w.Body.String()).To(ContainSubstring("test-renovator"))
			})
		})

		Describe("getGitRepos", func() {
			It("should return git repos with filtering and sorting parameters", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/gitrepos?namespace=test-namespace&sort=date&order=asc", nil)
				w := httptest.NewRecorder()

				handler.getGitRepos(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))
				Expect(w.Body.String()).To(ContainSubstring("test-repo"))
			})
		})

		Describe("getRunners", func() {
			It("should return runners with sorting parameters", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/runners?sort=name&order=asc", nil)
				w := httptest.NewRecorder()

				handler.getRunners(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))
				Expect(w.Body.String()).To(ContainSubstring("test-runner"))
			})
		})

		Describe("getDiscoveries", func() {
			It("should return discoveries with sorting parameters", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/discoveries?sort=date&order=desc", nil)
				w := httptest.NewRecorder()

				handler.getDiscoveries(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))
				Expect(w.Body.String()).To(ContainSubstring("test-discovery"))
			})
		})
	})
})
