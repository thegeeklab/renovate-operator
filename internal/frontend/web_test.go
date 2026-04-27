package frontend

import (
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("WebHandler", func() {
	var (
		fakeClient    client.Client
		fakeClientset *kubernetesfake.Clientset
		handler       *WebHandler
		scheme        *runtime.Scheme
		testObjects   []runtime.Object
		broker        *SSEBroker
		dummyAssets   FrontendAssets
		renovator     types.UID = "test-uid-123"
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
					UID:               renovator,
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
			},
			&renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: string(renovator),
					},
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
				Status: renovatev1beta1.GitRepoStatus{
					WebhookID: "12345",
				},
			},
			&renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: string(renovator),
					},
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
			},
			&renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "test-namespace",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: string(renovator),
					},
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
			},
		}

		fakeClientset = kubernetesfake.NewSimpleClientset()
		broker = NewSSEBroker()

		dummyAssets = FrontendAssets{
			Scripts: []string{"/static/assets/main-123.js"},
			Styles:  []string{"/static/assets/main-123.css"},
		}

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(testObjects...).Build()
		handler = NewWebHandler(fakeClient, fakeClientset, broker, dummyAssets)
	})

	Describe("NewWebHandler", func() {
		It("should create a new WebHandler", func() {
			Expect(handler).NotTo(BeNil())
			Expect(handler.dataFactory).NotTo(BeNil())
			Expect(handler.Broker).To(Equal(broker))
			Expect(handler.assets).To(Equal(dummyAssets))
		})
	})

	Describe("HandleDashboard", func() {
		It("should handle index requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()

			handler.HandleDashboard(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html"))
			Expect(w.Body.String()).To(ContainSubstring("test-renovator"))
		})

		It("should return partial for HTMX requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()

			handler.HandleDashboard(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).NotTo(ContainSubstring("<!DOCTYPE html>"))
		})
	})

	Describe("HandleGitReposPartial", func() {
		It("should handle git repos partial requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/gitrepos?namespace=test-namespace", nil)
			w := httptest.NewRecorder()

			handler.HandleGitReposPartial(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html"))
		})

		It("should handle git repos partial requests with sorting parameters", func() {
			req := httptest.NewRequest(
				http.MethodGet,
				"/gitrepos?namespace=test-namespace&sort=name&order=desc",
				nil,
			)
			w := httptest.NewRecorder()

			handler.HandleGitReposPartial(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html"))
		})

		It("should return bad request for missing namespace parameter", func() {
			req := httptest.NewRequest(http.MethodGet, "/gitrepos", nil)
			w := httptest.NewRecorder()

			handler.HandleGitReposPartial(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("HandleGitRepoView", func() {
		It("should return bad request for missing parameters", func() {
			req := httptest.NewRequest(http.MethodGet, "/gitrepo", nil)
			w := httptest.NewRecorder()

			handler.HandleGitRepoView(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should handle git repo view requests", func() {
			req := httptest.NewRequest(http.MethodGet, "/gitrepo?namespace=test-namespace&name=test-repo", nil)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()

			handler.HandleGitRepoView(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html"))
		})

		It("should handle git repo view requests with sorting parameters", func() {
			req := httptest.NewRequest(
				http.MethodGet,
				"/gitrepo?namespace=test-namespace&name=test-repo&sort=date&order=desc",
				nil,
			)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()

			handler.HandleGitRepoView(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html"))
		})
	})

	Describe("HandleJobLogs", func() {
		It("should return bad request for missing parameters", func() {
			req := httptest.NewRequest(http.MethodGet, "/joblogs", nil)
			w := httptest.NewRecorder()

			handler.HandleJobLogs(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should gracefully handle missing logs with an error message", func() {
			req := httptest.NewRequest(
				http.MethodGet,
				"/joblogs?namespace=test-namespace&runner=test-runner&job=missing-job",
				nil,
			)
			w := httptest.NewRecorder()

			handler.HandleJobLogs(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring("Logs are no longer available"))
		})
	})
})
