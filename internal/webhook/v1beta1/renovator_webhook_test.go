package v1beta1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Renovator Webhook", func() {
	var (
		obj       *renovatev1beta1.Renovator
		oldObj    *renovatev1beta1.Renovator
		defaulter RenovatorCustomDefaulter
		ctx       context.Context
	)

	BeforeEach(func() {
		obj = &renovatev1beta1.Renovator{}
		oldObj = &renovatev1beta1.Renovator{}
		defaulter = RenovatorCustomDefaulter{}
		ctx = context.Background()
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {
		// Clean up resources if needed
	})

	Context("When creating Renovator under Defaulting Webhook", func() {
		It("Should apply defaults when required fields are empty", func() {
			By("calling the Default method to apply defaults")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.Logging.Level).To(BeEquivalentTo(renovatev1beta1.LogLevel_INFO))
			Expect(obj.Spec.Suspend).NotTo(BeNil())
			Expect(*obj.Spec.Suspend).To(BeFalse())
			Expect(obj.Spec.Schedule).To(Equal(renovatev1beta1.DefaultSchedule))
			Expect(obj.Spec.Image).To(Equal(renovatev1beta1.DefaultOperatorContainerImage))
			Expect(obj.Spec.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			Expect(obj.Spec.Renovate.Image).To(Equal(renovatev1beta1.DefaultRenovateContainerImage))
			Expect(obj.Spec.Renovate.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		})

		It("Should not override existing values when defaults are applied", func() {
			By("setting some existing values")
			obj.Spec.Image = "custom-image:latest"
			obj.Spec.ImagePullPolicy = corev1.PullAlways
			obj.Spec.Logging.Level = renovatev1beta1.LogLevel_DEBUG
			defaultSuspend := true
			obj.Spec.Suspend = &defaultSuspend
			obj.Spec.Schedule = "0 */1 * * *"
			obj.Spec.Runner.Strategy = "rolling"
			obj.Spec.Runner.Instances = 3
			obj.Spec.Discovery.Schedule = "0 */1 * * *"
			obj.Spec.Renovate.Image = "custom-renovate:latest"
			obj.Spec.Renovate.ImagePullPolicy = corev1.PullAlways
			By("calling the Default method to apply defaults")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.Logging.Level).To(BeEquivalentTo(renovatev1beta1.LogLevel_DEBUG))
			Expect(obj.Spec.Suspend).NotTo(BeNil())
			Expect(*obj.Spec.Suspend).To(BeTrue())
			Expect(obj.Spec.Schedule).To(Equal("0 */1 * * *"))
			Expect(obj.Spec.Image).To(Equal("custom-image:latest"))
			Expect(obj.Spec.ImagePullPolicy).To(BeEquivalentTo(corev1.PullAlways))
			Expect(obj.Spec.Runner.Strategy).To(BeEquivalentTo("rolling"))
			Expect(obj.Spec.Runner.Instances).To(BeEquivalentTo(3))
			Expect(obj.Spec.Discovery.Schedule).To(Equal("0 */1 * * *"))
			Expect(obj.Spec.Renovate.Image).To(Equal("custom-renovate:latest"))
			Expect(obj.Spec.Renovate.ImagePullPolicy).To(Equal(corev1.PullAlways))
		})

		It("Should return error when object is not a Renovator", func() {
			By("calling the Default method with wrong object type")
			err := defaulter.Default(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected a Renovator object but got other type"))
		})
	})
})
