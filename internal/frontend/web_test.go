package frontend

import (
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/logstore"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("WebHandler", func() {
	var (
		k8sClient   client.Client
		handler     *WebHandler
		scheme      *runtime.Scheme
		testObjects []runtime.Object
		tempLogDir  string
		logManager  *logstore.Manager
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()

		err := renovatev1beta1.AddToScheme(scheme)
		Expect(err).NotTo(HaveOccurred())

		err = batchv1.AddToScheme(scheme)
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

		tempLogDir, err = os.MkdirTemp("", "operator-web-test-*")
		Expect(err).NotTo(HaveOccurred())

		k8sClientset := fakeclientset.NewSimpleClientset()
		fileStore := logstore.NewFileStore(tempLogDir)
		logManager = logstore.NewManager(k8sClientset, fileStore)

		k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(testObjects...).Build()
		handler = NewWebHandler(k8sClient, logManager)
	})

	AfterEach(func() {
		err := os.RemoveAll(tempLogDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("NewWebHandler", func() {
		It("should create a new WebHandler", func() {
			Expect(handler).NotTo(BeNil())
			Expect(handler.logManager).NotTo(BeNil())
		})
	})

	Describe("RegisterRoutes", func() {
		It("should register dashboard routes", func() {
			Expect(handler).NotTo(BeNil())
		})
	})

	Describe("HandleIndex", func() {
		It("should handle index requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()

			handler.HandleIndex(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html; charset=utf-8"))
		})
	})

	Describe("HandleRenovatorsPartial", func() {
		It("should handle renovators partial requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/partials/renovators", nil)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()

			handler.HandleRenovatorsPartial(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html"))
		})
	})

	Describe("HandleGitReposPartial", func() {
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

	Describe("HandleGitRepoView", func() {
		It("should return bad request for missing parameters", func() {
			req := httptest.NewRequest(http.MethodGet, "/partials/gitrepo", nil)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()

			handler.HandleGitRepoView(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should handle git repo view requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/partials/gitrepo?namespace=test-namespace&name=test-repo", nil)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()

			handler.HandleGitRepoView(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html"))
		})
	})

	Describe("HandleJobLogs", func() {
		It("should return bad request for missing parameters", func() {
			req := httptest.NewRequest(http.MethodGet, "/partials/joblogs", nil)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()

			handler.HandleJobLogs(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should gracefully handle missing logs with an error template", func() {
			req := httptest.NewRequest(
				http.MethodGet,
				"/partials/joblogs?namespace=test-namespace&runner=test-runner&job=missing-job",
				nil,
			)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()

			handler.HandleJobLogs(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring("Logs are no longer available"))
		})
	})

	Describe("EnsureHTMXRequest", func() {
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
