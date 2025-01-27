package util

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func HandleError(ctx context.Context, result *ctrl.Result, err error) (ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)

	if result != nil && err != nil {
		ctxLogger.Error(err, "Requeue", "after", result.RequeueAfter)

		return *result, err
	}

	if result != nil {
		return *result, nil
	}

	return ctrl.Result{}, err
}
