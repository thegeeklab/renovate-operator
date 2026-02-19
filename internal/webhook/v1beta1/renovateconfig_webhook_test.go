package v1beta1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("RenovateConfig Webhook", func() {
	var (
		obj       *renovatev1beta1.RenovateConfig
		oldObj    *renovatev1beta1.RenovateConfig
		defaulter RenovateConfigCustomDefaulter
		ctx       context.Context
	)

	BeforeEach(func() {
		obj = &renovatev1beta1.RenovateConfig{}
		oldObj = &renovatev1beta1.RenovateConfig{}
		defaulter = RenovateConfigCustomDefaulter{}
		ctx = context.Background()

		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {
		// Clean up resources if needed
	})

	Context("When creating RenovateConfig under Defaulting Webhook", func() {
		It("Should apply defaults when required fields are empty", func() {
			By("calling the Default method to apply defaults")

			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.Logging).NotTo(BeNil())
			Expect(obj.Spec.Logging.Level).To(BeEquivalentTo(renovatev1beta1.LogLevel_INFO))
			Expect(obj.Spec.Onboarding).NotTo(BeNil())
			Expect(*obj.Spec.Onboarding).To(BeTrue())
			Expect(obj.Spec.PrHourlyLimit).To(BeEquivalentTo(10))
			Expect(obj.Spec.Image).To(Equal(renovatev1beta1.DefaultRenovateContainerImage))
			Expect(obj.Spec.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		})

		It("Should not override existing values when defaults are applied", func() {
			By("setting some existing values")

			obj.Spec.Image = "custom-image:latest"
			obj.Spec.ImagePullPolicy = corev1.PullAlways
			defaultOnboarding := false
			obj.Spec.Onboarding = &defaultOnboarding
			obj.Spec.PrHourlyLimit = 20
			obj.Spec.Logging = &renovatev1beta1.LoggingSpec{
				Level: renovatev1beta1.LogLevel_DEBUG,
			}

			By("calling the Default method to apply defaults")

			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.Logging.Level).To(BeEquivalentTo(renovatev1beta1.LogLevel_DEBUG))
			Expect(obj.Spec.Onboarding).NotTo(BeNil())
			Expect(*obj.Spec.Onboarding).To(BeFalse())
			Expect(obj.Spec.PrHourlyLimit).To(BeEquivalentTo(20))
			Expect(obj.Spec.Image).To(Equal("custom-image:latest"))
			Expect(obj.Spec.ImagePullPolicy).To(Equal(corev1.PullAlways))
		})

		It("Should return error when object is not a RenovateConfig", func() {
			By("calling the Default method with wrong object type")

			err := defaulter.Default(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected a RenovateConfig object but got other type"))
		})
	})
})
