package runner

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("RunnerMetadata", func() {
	var request reconcile.Request

	BeforeEach(func() {
		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
	})

	It("should create runner metadata with correct name and namespace", func() {
		metadata := RunnerMetadata(request)
		Expect(metadata.Name).To(Equal("test-name-runner"))
		Expect(metadata.Namespace).To(Equal("test-namespace"))
	})

	It("should handle empty namespace in request", func() {
		request.Namespace = ""
		metadata := RunnerMetadata(request)
		Expect(metadata.Name).To(Equal("test-name-runner"))
		Expect(metadata.Namespace).To(BeEmpty())
	})
})

var _ = Describe("RunnerName", func() {
	var request reconcile.Request

	BeforeEach(func() {
		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: "test-name",
			},
		}
	})

	It("should create runner name with correct suffix", func() {
		name := RunnerName(request)
		Expect(name).To(Equal("test-name-runner"))
	})

	It("should handle empty name in request", func() {
		request.Name = ""
		name := RunnerName(request)
		Expect(name).To(Equal("-runner"))
	})

	It("should handle special characters in name", func() {
		request.Name = "test-name-with-special-chars"
		name := RunnerName(request)
		Expect(name).To(Equal("test-name-with-special-chars-runner"))
	})

	It("should handle very long names", func() {
		longName := "test-name-with-very-long-name-that-exceeds-normal-length"
		request.Name = longName
		name := RunnerName(request)
		Expect(name).To(Equal(longName + "-runner"))
	})
})
