package reconciler

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Results", func() {
	var results *Results

	BeforeEach(func() {
		results = &Results{}
	})

	Context("Collect", func() {
		It("should handle nil result", func() {
			results.Collect(nil)
			Expect(results.shouldRequeue).To(BeFalse())
			Expect(results.minRequeueAfter).To(BeZero())
		})

		It("should set shouldRequeue when Requeue is true", func() {
			results.Collect(&ctrl.Result{RequeueAfter: time.Second})
			Expect(results.shouldRequeue).To(BeTrue())
		})

		It("should keep shortest RequeueAfter duration", func() {
			results.Collect(&ctrl.Result{RequeueAfter: 5 * time.Minute})
			results.Collect(&ctrl.Result{RequeueAfter: 1 * time.Minute})
			results.Collect(&ctrl.Result{RequeueAfter: 10 * time.Minute})
			Expect(results.minRequeueAfter).To(Equal(1 * time.Minute))
		})

		It("should ignore zero RequeueAfter", func() {
			results.Collect(&ctrl.Result{RequeueAfter: 5 * time.Minute})
			results.Collect(&ctrl.Result{RequeueAfter: 0})
			Expect(results.minRequeueAfter).To(Equal(5 * time.Minute))
		})

		It("should maintain shouldRequeue state across multiple collections", func() {
			results.Collect(&ctrl.Result{RequeueAfter: time.Second})
			results.Collect(&ctrl.Result{RequeueAfter: 0})
			Expect(results.shouldRequeue).To(BeTrue())
		})
	})

	Context("ToResult", func() {
		It("should return ctrl.Result with collected values", func() {
			results.shouldRequeue = true
			results.minRequeueAfter = 5 * time.Minute
			result := results.ToResult()
			Expect(result.Requeue).To(BeTrue())
			Expect(result.RequeueAfter).To(Equal(5 * time.Minute))
		})

		It("should return ctrl.Result with zero values when nothing collected", func() {
			result := results.ToResult()
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())
		})
	})
})
