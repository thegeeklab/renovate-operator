package k8s

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateOrUpdate is a wrapper function that creates or updates a Kubernetes object
// with the specified owner and mutation function.
func CreateOrUpdate(
	ctx context.Context,
	c client.Client,
	obj, owner client.Object,
	mutate controllerutil.MutateFn,
) (controllerutil.OperationResult, error) {
	ctxLogger := log.FromContext(ctx)

	op, err := controllerutil.CreateOrUpdate(ctx, c, obj, func() error {
		if err := mutate(); err != nil {
			return err
		}

		if owner != nil {
			if err := controllerutil.SetControllerReference(owner, obj, c.Scheme()); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		ctxLogger.Error(
			err, "Failed to ensure resource",
			"resource", obj.GetObjectKind().GroupVersionKind().Kind, "operation", op,
		)

		return controllerutil.OperationResultNone, err
	}

	if op != controllerutil.OperationResultNone {
		ctxLogger.Info(
			"Resource reconciled",
			"resource", obj.GetObjectKind().GroupVersionKind().Kind, "operation", op,
		)
	}

	return op, nil
}
