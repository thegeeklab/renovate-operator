package renovator

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
)

var _ = Describe("Renovator Annotation Functions", func() {
	Describe("GetRenovatorOperations", func() {
		It("should return empty slice when no operation annotation exists", func() {
			annotations := map[string]string{
				"other-annotation": "some-value",
			}

			operations := GetRenovatorOperations(annotations)
			Expect(operations).To(BeEmpty())
		})

		It("should return single operation when annotation contains one operation", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover,
			}

			operations := GetRenovatorOperations(annotations)
			Expect(operations).To(HaveLen(1))
			Expect(operations[0]).To(Equal(renovatev1beta1.OperationDiscover))
		})

		It("should return multiple operations when annotation contains multiple operations", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover + ";" + "other-operation;third-operation",
			}

			operations := GetRenovatorOperations(annotations)
			Expect(operations).To(HaveLen(3))
			Expect(operations[0]).To(Equal(renovatev1beta1.OperationDiscover))
			Expect(operations[1]).To(Equal("other-operation"))
			Expect(operations[2]).To(Equal("third-operation"))
		})

		It("should trim whitespace from operations", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: "  " + renovatev1beta1.OperationDiscover + "  ;  other-operation  ",
			}

			operations := GetRenovatorOperations(annotations)
			Expect(operations).To(HaveLen(2))
			Expect(operations[0]).To(Equal(renovatev1beta1.OperationDiscover))
			Expect(operations[1]).To(Equal("other-operation"))
		})

		It("should return nil when annotation value is empty", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: "",
			}

			operations := GetRenovatorOperations(annotations)
			Expect(operations).To(BeNil())
		})
	})

	Describe("HasRenovatorOperationDiscover", func() {
		It("should return false when no operation annotation exists", func() {
			annotations := map[string]string{
				"other-annotation": "some-value",
			}

			hasDiscover := HasRenovatorOperationDiscover(annotations)
			Expect(hasDiscover).To(BeFalse())
		})

		It("should return true when discover operation is present", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover,
			}

			hasDiscover := HasRenovatorOperationDiscover(annotations)
			Expect(hasDiscover).To(BeTrue())
		})

		It("should return true when discover operation is among multiple operations", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: "other-operation;" + renovatev1beta1.OperationDiscover + ";third-operation",
			}

			hasDiscover := HasRenovatorOperationDiscover(annotations)
			Expect(hasDiscover).To(BeTrue())
		})

		It("should return false when discover operation is not present among multiple operations", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: "other-operation;third-operation",
			}

			hasDiscover := HasRenovatorOperationDiscover(annotations)
			Expect(hasDiscover).To(BeFalse())
		})

		It("should return false when annotation value is empty", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: "",
			}

			hasDiscover := HasRenovatorOperationDiscover(annotations)
			Expect(hasDiscover).To(BeFalse())
		})
	})

	Describe("HasRenovatorOperationRenovate", func() {
		It("should return false when no operation annotation exists", func() {
			annotations := map[string]string{
				"other-annotation": "some-value",
			}

			hasRenovate := HasRenovatorOperationRenovate(annotations)
			Expect(hasRenovate).To(BeFalse())
		})

		It("should return true when renovate operation is present", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationRenovate,
			}

			hasRenovate := HasRenovatorOperationRenovate(annotations)
			Expect(hasRenovate).To(BeTrue())
		})

		It("should return true when renovate operation is among multiple operations", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: "other-operation;" + renovatev1beta1.OperationRenovate + ";third-operation",
			}

			hasRenovate := HasRenovatorOperationRenovate(annotations)
			Expect(hasRenovate).To(BeTrue())
		})

		It("should return false when renovate operation is not present among multiple operations", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: "other-operation;third-operation",
			}

			hasRenovate := HasRenovatorOperationRenovate(annotations)
			Expect(hasRenovate).To(BeFalse())
		})

		It("should return false when annotation value is empty", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: "",
			}

			hasRenovate := HasRenovatorOperationRenovate(annotations)
			Expect(hasRenovate).To(BeFalse())
		})
	})

	Describe("HasRenovatorOperation", func() {
		It("should return false when no operation annotation exists", func() {
			annotations := map[string]string{
				"other-annotation": "some-value",
			}

			hasOperation := HasRenovatorOperation(annotations)
			Expect(hasOperation).To(BeFalse())
		})

		It("should return true when operation annotation exists", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover,
			}

			hasOperation := HasRenovatorOperation(annotations)
			Expect(hasOperation).To(BeTrue())
		})

		It("should return true when multiple operations are present", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover + ";" + renovatev1beta1.OperationRenovate,
			}

			hasOperation := HasRenovatorOperation(annotations)
			Expect(hasOperation).To(BeTrue())
		})

		It("should return false when annotation value is empty", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: "",
			}

			hasOperation := HasRenovatorOperation(annotations)
			Expect(hasOperation).To(BeFalse())
		})
	})

	Describe("RemoveRenovatorOperation", func() {
		It("should remove operation annotation when it exists", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover,
				"other-annotation":                 "some-value",
			}

			result := RemoveRenovatorOperation(annotations)
			Expect(result).To(HaveLen(1))
			Expect(result).NotTo(HaveKey(renovatev1beta1.RenovatorOperation))
			Expect(result).To(HaveKey("other-annotation"))
			Expect(result["other-annotation"]).To(Equal("some-value"))
		})

		It("should not modify other annotations", func() {
			annotations := map[string]string{
				renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover,
				"annotation1":                      "value1",
				"annotation2":                      "value2",
			}

			result := RemoveRenovatorOperation(annotations)
			Expect(result).To(HaveLen(2))
			Expect(result).NotTo(HaveKey(renovatev1beta1.RenovatorOperation))
			Expect(result["annotation1"]).To(Equal("value1"))
			Expect(result["annotation2"]).To(Equal("value2"))
		})

		It("should handle nil annotations", func() {
			var annotations map[string]string

			result := RemoveRenovatorOperation(annotations)
			Expect(result).NotTo(BeNil())
			Expect(result).To(BeEmpty())
		})

		It("should handle empty annotations", func() {
			annotations := map[string]string{}

			result := RemoveRenovatorOperation(annotations)
			Expect(result).NotTo(BeNil())
			Expect(result).To(BeEmpty())
		})

		It("should not fail when operation annotation does not exist", func() {
			annotations := map[string]string{
				"other-annotation": "some-value",
			}

			result := RemoveRenovatorOperation(annotations)
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKey("other-annotation"))
			Expect(result["other-annotation"]).To(Equal("some-value"))
		})
	})
})
