package job

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
)

var _ = Describe("FailureReason", func() {
	It("should return the reason from a Failed condition", func() {
		job := &batchv1.Job{
			Name:      "test-job",
			Namespace: "default",
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobFailed,
						Reason: "BackoffLimitExceeded",
					},
				},
			},
		}

		Expect(FailureReason(job)).To(Equal("BackoffLimitExceeded"))
	})

	It("should return the first matching reason when multiple conditions exist", func() {
		job := &batchv1.Job{
			Name:      "test-job",
			Namespace: "default",
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobComplete,
						Reason: "Completed",
					},
					{
						Type:   batchv1.JobFailed,
						Reason: "DeadlineExceeded",
					},
					{
						Type:   batchv1.JobFailed,
						Reason: "BackoffLimitExceeded",
					},
				},
			},
		}

		Expect(FailureReason(job)).To(Equal("DeadlineExceeded"))
	})

	It("should return unknown when the Failed condition has an empty reason", func() {
		job := &batchv1.Job{
			Name:      "test-job",
			Namespace: "default",
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobFailed,
						Reason: "",
					},
				},
			},
		}

		Expect(FailureReason(job)).To(Equal("unknown"))
	})

	It("should return unknown when there are no conditions", func() {
		job := &batchv1.Job{
			Name:      "test-job",
			Namespace: "default",
			Status:    batchv1.JobStatus{},
		}

		Expect(FailureReason(job)).To(Equal("unknown"))
	})

	It("should return unknown when there is no Failed condition", func() {
		job := &batchv1.Job{
			Name:      "test-job",
			Namespace: "default",
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobComplete,
						Reason: "Completed",
					},
				},
			},
		}

		Expect(FailureReason(job)).To(Equal("unknown"))
	})
})
