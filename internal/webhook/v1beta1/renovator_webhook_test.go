package v1beta1

import (
	"context"
	"strings"

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

		It("Should set LabelAuthProvider when authProviderRef is set", func() {
			By("setting authProviderRef")

			obj.Spec.AuthProviderRef = "test-auth-provider"

			By("calling the Default method")

			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Labels).NotTo(BeNil())
			Expect(obj.Labels[renovatev1beta1.LabelAuthProvider]).To(Equal("test-auth-provider"))
		})

		It("Should remove LabelAuthProvider when authProviderRef is empty", func() {
			By("setting authProviderRef and then clearing it")

			obj.Spec.AuthProviderRef = "test-auth-provider"
			obj.Labels = map[string]string{
				renovatev1beta1.LabelAuthProvider: "test-auth-provider",
			}

			By("clearing authProviderRef")

			obj.Spec.AuthProviderRef = ""

			By("calling the Default method")

			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			_, exists := obj.Labels[renovatev1beta1.LabelAuthProvider]
			Expect(exists).To(BeFalse())
		})

		It("Should override manually set LabelAuthProvider", func() {
			By("manually setting LabelAuthProvider to wrong value")

			obj.Spec.AuthProviderRef = "correct-auth-provider"
			obj.Labels = map[string]string{
				renovatev1beta1.LabelAuthProvider: "wrong-auth-provider",
			}

			By("calling the Default method")

			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Labels[renovatev1beta1.LabelAuthProvider]).To(Equal("correct-auth-provider"))
		})

		It("Should truncate LabelAuthProvider when authProviderRef exceeds 63 characters", func() {
			By("setting authProviderRef to a very long name")

			longName := strings.Repeat("a", 100)
			obj.Spec.AuthProviderRef = longName

			By("calling the Default method")

			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Labels).NotTo(BeNil())
			Expect(len(obj.Labels[renovatev1beta1.LabelAuthProvider])).To(BeNumerically("<=", 63))
		})

		It("Should default scratch volume path when empty", func() {
			By("setting scratch volume without path")

			obj.Spec.ScratchVolume = &renovatev1beta1.ScratchVolumeSpec{
				Medium: corev1.StorageMediumMemory,
			}

			By("calling the Default method")

			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.ScratchVolume.Path).To(Equal(renovatev1beta1.DefaultScratchVolumePath))
		})

		It("Should not override existing scratch volume path", func() {
			By("setting scratch volume with custom path")

			obj.Spec.ScratchVolume = &renovatev1beta1.ScratchVolumeSpec{
				Path:   "/custom/path",
				Medium: corev1.StorageMediumMemory,
			}

			By("calling the Default method")

			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.ScratchVolume.Path).To(Equal("/custom/path"))
		})
	})

	Context("When creating Renovator under Validating Webhook", func() {
		var validator RenovatorCustomValidator

		BeforeEach(func() {
			validator = RenovatorCustomValidator{}
		})

		It("Should accept valid timezone", func() {
			By("setting a valid timezone")

			obj.Spec.Timezone = "America/New_York"

			By("calling the ValidateCreate method")

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should accept empty timezone", func() {
			By("leaving timezone empty")

			obj.Spec.Timezone = ""

			By("calling the ValidateCreate method")

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should reject invalid timezone", func() {
			By("setting an invalid timezone")

			obj.Spec.Timezone = "Invalid/Timezone"

			By("calling the ValidateCreate method")

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid timezone"))
			Expect(warnings).To(BeNil())
		})

		It("Should validate timezone in discovery spec", func() {
			By("setting an invalid timezone in discovery")

			obj.Spec.Discovery.Timezone = "Invalid/Timezone"

			By("calling the ValidateCreate method")

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid timezone"))
			Expect(warnings).To(BeNil())
		})

		It("Should validate timezone in runner spec", func() {
			By("setting an invalid timezone in runner")

			obj.Spec.Runner.Timezone = "Invalid/Timezone"

			By("calling the ValidateCreate method")

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid timezone"))
			Expect(warnings).To(BeNil())
		})

		It("Should return error when object is nil on ValidateCreate", func() {
			By("calling the ValidateCreate method with nil object")

			warnings, err := validator.ValidateCreate(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected a Renovator object but got other type"))
			Expect(warnings).To(BeNil())
		})

		It("Should validate timezone on update", func() {
			By("setting an invalid timezone")

			obj.Spec.Timezone = "Invalid/Timezone"

			By("calling the ValidateUpdate method")

			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid timezone"))
			Expect(warnings).To(BeNil())
		})

		It("Should accept valid timezone on update", func() {
			By("setting a valid timezone")

			obj.Spec.Timezone = "America/New_York"

			By("calling the ValidateUpdate method")

			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should accept valid absolute scratch volume path", func() {
			By("setting a valid absolute path")

			obj.Spec.ScratchVolume = &renovatev1beta1.ScratchVolumeSpec{
				Path: "/tmp/renovate",
			}

			By("calling the ValidateCreate method")

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should reject relative scratch volume path", func() {
			By("setting a relative path")

			obj.Spec.ScratchVolume = &renovatev1beta1.ScratchVolumeSpec{
				Path: "relative/path",
			}

			By("calling the ValidateCreate method")

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("path must be absolute"))
			Expect(warnings).To(BeNil())
		})

		It("Should accept empty scratch volume path", func() {
			By("leaving path empty (will be defaulted)")

			obj.Spec.ScratchVolume = &renovatev1beta1.ScratchVolumeSpec{
				Path: "",
			}

			By("calling the ValidateCreate method")

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should return error when object is nil on ValidateUpdate", func() {
			By("calling the ValidateUpdate method with nil object")

			warnings, err := validator.ValidateUpdate(ctx, oldObj, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected a Renovator object but got other type"))
			Expect(warnings).To(BeNil())
		})

		It("Should accept valid object on ValidateDelete", func() {
			By("calling the ValidateDelete method")

			warnings, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should return error when object is nil on ValidateDelete", func() {
			By("calling the ValidateDelete method with nil object")

			warnings, err := validator.ValidateDelete(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected a Renovator object but got other type"))
			Expect(warnings).To(BeNil())
		})
	})
})
