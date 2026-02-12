package frontend

import (
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/thegeeklab/renovate-operator/api/v1beta1"
)

var _ = Describe("APIHandler", func() {
	var (
		client      client.Client
		handler     *APIHandler
		scheme      *runtime.Scheme
		testObjects []runtime.Object
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		err := v1beta1.AddToScheme(scheme)
		Expect(err).NotTo(HaveOccurred())

		testObjects = []runtime.Object{
			&v1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "test-namespace",
				},
				Status: v1beta1.RenovatorStatus{
					Ready: true,
				},
			},
			&v1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.GitRepoSpec{
					WebhookID: "12345",
				},
				Status: v1beta1.GitRepoStatus{
					Ready: true,
				},
			},
			&v1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.RunnerSpec{
					Strategy:  v1beta1.RunnerStrategy_NONE,
					Instances: 1,
				},
				Status: v1beta1.RunnerStatus{
					Ready: true,
				},
			},
			&v1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "test-namespace",
				},
				Status: v1beta1.DiscoveryStatus{
					Ready: true,
				},
			},
		}

		client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(testObjects...).Build()
		handler = NewAPIHandler(client)
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
})
