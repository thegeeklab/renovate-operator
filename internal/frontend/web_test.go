package frontend

import (
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("WebHandler", func() {
	var (
		client      client.Client
		handler     *WebHandler
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
					Name:              "test-renovator",
					Namespace:         "test-namespace",
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
				Status: renovatev1beta1.RenovatorStatus{
					Ready: true,
				},
			},
			&renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-repo",
					Namespace:         "test-namespace",
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
				Spec: renovatev1beta1.GitRepoSpec{
					WebhookID: "12345",
				},
				Status: renovatev1beta1.GitRepoStatus{
					Ready: true,
				},
			},
			&renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-runner",
					Namespace:         "test-namespace",
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
				Spec: renovatev1beta1.RunnerSpec{
					Instances: 1,
				},
				Status: renovatev1beta1.RunnerStatus{
					Ready: true,
				},
			},
			&renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-discovery",
					Namespace:         "test-namespace",
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
				Status: renovatev1beta1.DiscoveryStatus{
					Ready: true,
				},
			},
		}

		client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(testObjects...).Build()
		handler = NewWebHandler(client)
	})

	Describe("NewWebHandler", func() {
		It("should create a new WebHandler", func() {
			Expect(handler).NotTo(BeNil())
		})
	})

	Describe("RegisterRoutes", func() {
		It("should register dashboard routes", func() {
			Expect(handler).NotTo(BeNil())
		})
	})

	Describe("handleIndex", func() {
		It("should handle index requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()

			handler.HandleIndex(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html; charset=utf-8"))
		})
	})

	Describe("handleRenovatorsPartial", func() {
		It("should handle renovators partial requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/partials/renovators", nil)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()

			handler.HandleRenovatorsPartial(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html"))
		})
	})

	Describe("handleGitReposPartial", func() {
		It("should handle git repos partial requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/partials/gitrepos?namespace=test-namespace", nil)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()

			handler.HandleGitReposPartial(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html"))
		})

		It("should return bad request for missing namespace parameter", func() {
			req := httptest.NewRequest(http.MethodGet, "/partials/gitrepos", nil)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()

			handler.HandleGitReposPartial(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("ensureHTMXRequest", func() {
		It("should redirect non-HTMX requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/partials/test", nil)
			w := httptest.NewRecorder()

			dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			wrappedHandler := handler.EnsureHTMXRequest(dummyHandler)

			wrappedHandler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusFound))
		})

		It("should allow HTMX requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/partials/test", nil)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()

			dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			wrappedHandler := handler.EnsureHTMXRequest(dummyHandler)

			wrappedHandler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})
})
