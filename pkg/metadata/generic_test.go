package metadata

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("GenericMetadata", func() {
	var request reconcile.Request

	BeforeEach(func() {
		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
	})

	It("should create metadata with correct name and namespace", func() {
		metadata := GenericMetadata(request)
		Expect(metadata.Name).To(Equal("test-name"))
		Expect(metadata.Namespace).To(Equal("test-namespace"))
	})

	It("should handle empty name and namespace", func() {
		request.Name = ""
		request.Namespace = ""
		metadata := GenericMetadata(request)
		Expect(metadata.Name).To(BeEmpty())
		Expect(metadata.Namespace).To(BeEmpty())
	})

	It("should allow setting a custom name suffix", func() {
		metadata := GenericMetadata(request, "suffix")
		Expect(metadata.Name).To(Equal("test-name-suffix"))
	})

	It("should handle empty suffix", func() {
		metadata := GenericMetadata(request, "")
		Expect(metadata.Name).To(Equal("test-name"))
	})
})
