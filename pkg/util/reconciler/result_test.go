package reconciler

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Results", func() {
	Describe("Collect", func() {
		It("should handle nil result", func() {
			results := &Results{}
			results.Collect(nil)
			Expect(results.shouldRequeue).To(BeFalse())
			Expect(results.minRequeueAfter).To(Equal(time.Duration(0)))
		})

		It("should collect single result with requeue", func() {
			results := &Results{}
			result := &ctrl.Result{RequeueAfter: time.Minute}
			results.Collect(result)
			Expect(results.shouldRequeue).To(BeTrue())
			Expect(results.minRequeueAfter).To(Equal(time.Minute))
		})

		It("should collect single result without requeue", func() {
			results := &Results{}
			result := &ctrl.Result{}
			results.Collect(result)
			Expect(results.shouldRequeue).To(BeFalse())
			Expect(results.minRequeueAfter).To(Equal(time.Duration(0)))
		})

		It("should collect multiple results and find minimum requeue time", func() {
			results := &Results{}
			results.Collect(&ctrl.Result{RequeueAfter: time.Hour})
			results.Collect(&ctrl.Result{RequeueAfter: time.Minute})
			results.Collect(&ctrl.Result{RequeueAfter: time.Second})
			Expect(results.shouldRequeue).To(BeTrue())
			Expect(results.minRequeueAfter).To(Equal(time.Second))
		})

		It("should set shouldRequeue when any result has requeue", func() {
			results := &Results{}
			results.Collect(&ctrl.Result{})
			results.Collect(&ctrl.Result{RequeueAfter: time.Minute})
			results.Collect(&ctrl.Result{})
			Expect(results.shouldRequeue).To(BeTrue())
			Expect(results.minRequeueAfter).To(Equal(time.Minute))
		})

		It("should ignore zero requeue times", func() {
			results := &Results{}
			results.Collect(&ctrl.Result{RequeueAfter: 0})
			results.Collect(&ctrl.Result{RequeueAfter: time.Minute})
			Expect(results.shouldRequeue).To(BeTrue())
			Expect(results.minRequeueAfter).To(Equal(time.Minute))
		})
	})

	Describe("ToResult", func() {
		It("should return result with requeue when shouldRequeue is true", func() {
			results := &Results{
				shouldRequeue:   true,
				minRequeueAfter: time.Minute,
			}
			result := results.ToResult()
			Expect(result.Requeue).To(BeTrue())
			Expect(result.RequeueAfter).To(Equal(time.Minute))
		})

		It("should return result without requeue when shouldRequeue is false", func() {
			results := &Results{
				shouldRequeue:   false,
				minRequeueAfter: 0,
			}
			result := results.ToResult()
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
		})

		It("should return result with zero requeue time when shouldRequeue is true but minRequeueAfter is zero", func() {
			results := &Results{
				shouldRequeue:   true,
				minRequeueAfter: 0,
			}
			result := results.ToResult()
			Expect(result.Requeue).To(BeTrue())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
		})
	})
})
