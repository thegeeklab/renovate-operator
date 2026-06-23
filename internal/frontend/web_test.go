package frontend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
		handler = NewWebHandler(fakeClient, fakeClientset, broker, dummyAssets, nil)
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

	Describe("HandleRenovatorCount", func() {
		It("should handle renovator count requests", func() {
			req := httptest.NewRequest(
				http.MethodGet,
				"/renovators/count?namespace=test-namespace&renovator=test-uid",
				nil,
			)
			w := httptest.NewRecorder()

			handler.HandleRenovatorCount(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html"))
		})

		It("should return bad request for missing namespace parameter", func() {
			req := httptest.NewRequest(http.MethodGet, "/renovators/count?renovator=test-uid", nil)
			w := httptest.NewRecorder()

			handler.HandleRenovatorCount(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return bad request for missing renovator parameter", func() {
			req := httptest.NewRequest(http.MethodGet, "/renovators/count?namespace=test-namespace", nil)
			w := httptest.NewRecorder()

			handler.HandleRenovatorCount(w, req)

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

		It("should return not found for non-existent repo", func() {
			req := httptest.NewRequest(http.MethodGet, "/gitrepo?namespace=test-namespace&name=nonexistent", nil)
			w := httptest.NewRecorder()

			handler.HandleGitRepoView(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
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
			Expect(w.Body.String()).To(ContainSubstring("Failed to fetch logs"))
		})
	})

	Describe("HandleDashboard search", func() {
		It("should handle search requests and return matching repos", func() {
			req := httptest.NewRequest(http.MethodGet, "/?search=test-repo", nil)
			w := httptest.NewRecorder()

			handler.HandleDashboard(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html"))
			Expect(w.Body.String()).To(ContainSubstring("test-repo"))
		})

		It("should return no results found for non-matching search", func() {
			req := httptest.NewRequest(http.MethodGet, "/?search=nonexistent", nil)
			w := httptest.NewRecorder()

			handler.HandleDashboard(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring("No Results Found"))
		})
	})

	Describe("HandleJobLogsDownload", func() {
		It("should return bad request for missing parameters", func() {
			req := httptest.NewRequest(http.MethodGet, "/joblogs/download", nil)
			w := httptest.NewRecorder()

			handler.HandleJobLogsDownload(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return not found for non-existent job", func() {
			req := httptest.NewRequest(http.MethodGet, "/joblogs/download?namespace=test-namespace&job=missing-job", nil)
			w := httptest.NewRecorder()

			handler.HandleJobLogsDownload(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
			Expect(w.Body.String()).To(ContainSubstring("Logs are no longer available"))
		})
	})

	Describe("isJobRunning", func() {
		It("should return false when job does not exist", func() {
			running := handler.isJobRunning(context.Background(), "test-namespace", "non-existent-job")
			Expect(running).To(BeFalse())
		})

		It("should return true when job has no completion time and no terminal conditions", func() {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "running-job", Namespace: "test-namespace"},
				Status:     batchv1.JobStatus{},
			}
			Expect(fakeClient.Create(context.Background(), job)).To(Succeed())

			running := handler.isJobRunning(context.Background(), "test-namespace", "running-job")
			Expect(running).To(BeTrue())
		})

		It("should return false when job has CompletionTime", func() {
			now := metav1.Now()
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "completed-job", Namespace: "test-namespace"},
				Status:     batchv1.JobStatus{CompletionTime: &now},
			}
			Expect(fakeClient.Create(context.Background(), job)).To(Succeed())

			running := handler.isJobRunning(context.Background(), "test-namespace", "completed-job")
			Expect(running).To(BeFalse())
		})

		It("should return false when job has JobFailed condition", func() {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "failed-job", Namespace: "test-namespace"},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
					},
				},
			}
			Expect(fakeClient.Create(context.Background(), job)).To(Succeed())

			running := handler.isJobRunning(context.Background(), "test-namespace", "failed-job")
			Expect(running).To(BeFalse())
		})
	})
})
