package reconciler

import (
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

type Results struct {
	shouldRequeue   bool
	minRequeueAfter time.Duration
}

func (r *Results) Collect(res *ctrl.Result) {
	if res == nil {
		return
	}

	r.shouldRequeue = r.shouldRequeue || res.Requeue
	if res.RequeueAfter > 0 {
		if r.minRequeueAfter == 0 || res.RequeueAfter < r.minRequeueAfter {
			r.minRequeueAfter = res.RequeueAfter
		}
	}
}

func (r *Results) ToResult() *ctrl.Result {
	return &ctrl.Result{
		Requeue:      r.shouldRequeue,
		RequeueAfter: r.minRequeueAfter,
	}
}
