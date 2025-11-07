package reconciler

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type GenericReconciler struct {
	KubeClient client.Client
	Scheme     *runtime.Scheme
	Req        ctrl.Request
}

func (r *GenericReconciler) ReconcileResource(
	ctx context.Context,
	current, expected client.Object,
) (*ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)

	key := client.ObjectKey{
		Namespace: expected.GetNamespace(),
		Name:      expected.GetName(),
	}
	gvk := expected.GetObjectKind().GroupVersionKind()

	current.GetObjectKind().SetGroupVersionKind(gvk)

	err := r.KubeClient.Get(ctx, key, current)

	resourceKind := current.GetObjectKind().GroupVersionKind().Kind
	resourceName := current.GetName()

	if err != nil {
		if errors.IsNotFound(err) {
			if err = r.KubeClient.Create(ctx, expected); err != nil {
				ctxLogger.Error(err, fmt.Sprintf("Failed to create %s", resourceKind), "resourceName", resourceName)

				return nil, err
			}

			ctxLogger.Info(fmt.Sprintf("Created %s", resourceKind), "resourceName", resourceName)

			return &ctrl.Result{Requeue: true}, nil
		}

		return nil, err
	}

	// Use Kubernetes' built-in equality function for comparison
	if !equality.Semantic.DeepDerivative(expected, current) {
		diff := cmp.Diff(current, expected)

		ctxLogger.V(1).Info("Resource differs",
			"resourceKind", resourceKind,
			"resourceName", resourceName,
			"diff", diff,
		)

		ctxLogger.Info(fmt.Sprintf("Updating %s", resourceKind), "resourceName", resourceName)

		if err = r.KubeClient.Update(ctx, expected); err != nil {
			ctxLogger.Error(err, "Failed to update", "resourceName", resourceName)

			return nil, err
		}

		ctxLogger.Info(fmt.Sprintf("Updated %s", resourceKind), "resourceName", resourceName)

		return &ctrl.Result{Requeue: true}, nil
	}

	return &ctrl.Result{}, nil
}
