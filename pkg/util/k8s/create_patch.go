package k8s

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateOrUpdate is a wrapper function that creates or updates a Kubernetes object
// with the specified owner and mutation function.
func CreateOrPatch(
	ctx context.Context,
	c client.Client,
	obj, owner client.Object,
	mutate controllerutil.MutateFn,
) (controllerutil.OperationResult, error) {
	log := logf.FromContext(ctx)

	op, err := createOrPatch(ctx, c, obj, func() error {
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
			err, fmt.Sprintf("Failed to reconcile object %#q", client.ObjectKeyFromObject(obj).String()),
			"kind", GVK(c.Scheme(), obj).Kind,
			"operation", op,
		)

		return controllerutil.OperationResultNone, err
	}

	if op != controllerutil.OperationResultNone {
		log.Info(
			fmt.Sprintf("Reconciled object %#q", client.ObjectKeyFromObject(obj).String()),
			"kind", GVK(c.Scheme(), obj).Kind,
			"operation", op,
		)
	}

	return op, nil
}

func createOrPatch(
	ctx context.Context,
	c client.Client,
	obj client.Object,
	mutate controllerutil.MutateFn,
) (controllerutil.OperationResult, error) {
	op := controllerutil.OperationResultNone

	errGet := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if client.IgnoreNotFound(errGet) != nil {
		return op, fmt.Errorf("failed get: %w", errGet)
	}

	old := obj.DeepCopyObject()

	if err := mutate(); err != nil {
		return op, fmt.Errorf("failed update: %w", err)
	}

	if api_errors.IsNotFound(errGet) {
		if err := c.Create(ctx, obj); err != nil {
			return op, fmt.Errorf("failed create: %w", err)
		}

		op = controllerutil.OperationResultCreated
	} else {
		//nolint:forcetypeassert
		if err := c.Patch(ctx, obj, client.MergeFrom(old.(client.Object))); err != nil {
			return op, fmt.Errorf("failed patch: %w", err)
		}

		op = controllerutil.OperationResultUpdated

		if equality.Semantic.DeepEqual(old, obj) {
			op = controllerutil.OperationResultNone
		}
	}

	return op, nil
}
