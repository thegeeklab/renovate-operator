package k8s

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateOrUpdate is a wrapper function that creates or updates a Kubernetes object
// with the specified owner and mutation function.
func CreateOrUpdate(
	ctx context.Context,
	c client.Client,
	obj, owner client.Object,
	mutate controllerutil.MutateFn,
) (controllerutil.OperationResult, error) {
	log := logf.FromContext(ctx)

	op, err := controllerutil.CreateOrUpdate(ctx, c, obj, func() error {
		if mutate != nil {
			if err := mutate(); err != nil {
				return err
			}
		}

		if owner != nil {
			if err := controllerutil.SetControllerReference(owner, obj, c.Scheme()); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		log.Error(
			err, "Failed to reconcile object",
			"object", client.ObjectKeyFromObject(obj).String(),
			"kind", GVK(c.Scheme(), obj).Kind,
			"operation", op,
		)

		return op, err
	}

	if op != controllerutil.OperationResultNone {
		log.Info(
			"Reconciled object",
			"object", client.ObjectKeyFromObject(obj).String(),
			"kind", GVK(c.Scheme(), obj).Kind,
			"operation", op,
		)
	}

	return op, nil
}
