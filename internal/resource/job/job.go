package job

import (
	batchv1 "k8s.io/api/batch/v1"
)

func FailureReason(j *batchv1.Job) string {
	for _, cond := range j.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Reason != "" {
			return cond.Reason
		}
	}

	return "unknown"
}
