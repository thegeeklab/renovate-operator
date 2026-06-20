package frontend

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-chi/chi/v5"
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
		handler = NewAPIHandler(fakeClient, nil, nil)
	})

	Describe("NewAPIHandler", func() {
		It("should create a new APIHandler", func() {
			Expect(handler).NotTo(BeNil())
		})
	})

	Describe("RegisterRoutes", func() {
		It("should register API routes", func() {
			router := chi.NewRouter()

			handler.RegisterRoutes(router)

			testCases := []struct {
				method string
				path   string
			}{
				{http.MethodGet, "/api/v1/version"},
				{http.MethodGet, "/api/v1/renovators"},
				{http.MethodGet, "/api/v1/gitrepos"},
				{http.MethodGet, "/api/v1/runners"},
				{http.MethodGet, "/api/v1/discoveries"},
				{http.MethodPost, "/api/v1/discovery/start"},
				{http.MethodGet, "/api/v1/discovery/status"},
			}

			for _, tc := range testCases {
				req := httptest.NewRequest(tc.method, tc.path, nil)
				w := httptest.NewRecorder()

				router.ServeHTTP(w, req)

				Expect(w.Code).NotTo(Equal(http.StatusNotFound), "Route %s %s should be registered", tc.method, tc.path)
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

			It("should include lastRenovateAt and lastRenovateStatus fields", func() {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/gitrepos", nil)
				w := httptest.NewRecorder()

				handler.getGitRepos(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(ContainSubstring("lastRenovateAt"))
				Expect(w.Body.String()).To(ContainSubstring("lastRenovateStatus"))
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
