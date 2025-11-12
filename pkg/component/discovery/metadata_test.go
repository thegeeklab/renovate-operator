package discovery

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("DiscoveryMetaData", func() {
	var request reconcile.Request

	BeforeEach(func() {
		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
	})

	It("should create discovery metadata with correct name and namespace", func() {
		metadata := DiscoveryMetaData(request)
		Expect(metadata.Name).To(Equal("test-name-discovery"))
		Expect(metadata.Namespace).To(Equal("test-namespace"))
	})

	It("should handle empty namespace in request", func() {
		request.Namespace = ""
		metadata := DiscoveryMetaData(request)
		Expect(metadata.Name).To(Equal("test-name-discovery"))
		Expect(metadata.Namespace).To(BeEmpty())
	})
})

var _ = Describe("DiscoveryName", func() {
	var request reconcile.Request

	BeforeEach(func() {
		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: "test-name",
			},
		}
	})

	It("should create discovery name with correct suffix", func() {
		name := DiscoveryName(request)
		Expect(name).To(Equal("test-name-discovery"))
	})

	It("should handle empty name in request", func() {
		request.Name = ""
		name := DiscoveryName(request)
		Expect(name).To(Equal("-discovery"))
	})
})
