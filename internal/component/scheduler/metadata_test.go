package scheduler

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("SchedulerMetadata", func() {
	var request reconcile.Request

	BeforeEach(func() {
		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
	})

	It("should create scheduler metadata with correct name and namespace", func() {
		metadata := SchedulerMetadata(request)
		Expect(metadata.Name).To(Equal("test-name-scheduler"))
		Expect(metadata.Namespace).To(Equal("test-namespace"))
	})

	It("should handle empty namespace in request", func() {
		request.Namespace = ""
		metadata := SchedulerMetadata(request)
		Expect(metadata.Name).To(Equal("test-name-scheduler"))
		Expect(metadata.Namespace).To(BeEmpty())
	})
})

var _ = Describe("SchedulerName", func() {
	var request reconcile.Request

	BeforeEach(func() {
		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: "test-name",
			},
		}
	})

	It("should create scheduler name with correct suffix", func() {
		name := SchedulerName(request)
		Expect(name).To(Equal("test-name-scheduler"))
	})

	It("should handle empty name in request", func() {
		request.Name = ""
		name := SchedulerName(request)
		Expect(name).To(Equal("-scheduler"))
	})

	It("should handle special characters in name", func() {
		request.Name = "test-name-with-special-chars"
		name := SchedulerName(request)
		Expect(name).To(Equal("test-name-with-special-chars-scheduler"))
	})

	It("should handle very long names", func() {
		longName := "test-name-with-very-long-name-that-exceeds-normal-length"
		request.Name = longName
		name := SchedulerName(request)
		Expect(name).To(Equal(longName + "-scheduler"))
	})
})
