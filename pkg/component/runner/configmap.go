package runner

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *RunnerReconciler) reconcileConfigMap(ctx context.Context) (*ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)

	obj, err := r.createConfigMap()
	if err != nil {
		return &ctrl.Result{}, err
	}

	op, err := ctrl.CreateOrUpdate(ctx, r.Client, obj, func() error {
		return nil
	})
	if err != nil {
		return &ctrl.Result{}, err
	}

	ctxLogger.V(1).Info("Runner ConfigMap", "object", client.ObjectKeyFromObject(obj), "operation", op)

	return &ctrl.Result{}, nil
}

func (r *RunnerReconciler) createConfigMap() (*corev1.ConfigMap, error) {
	renovateConfig := &Renovate{
		DryRun:        r.Instance.Spec.Renovate.DryRun,
		Onboarding:    *r.Instance.Spec.Renovate.Onboarding,
		PrHourlyLimit: r.Instance.Spec.Renovate.PrHourlyLimit,
		AddLabels:     r.Instance.Spec.Renovate.AddLabels,
		Platform:      r.Instance.Spec.Renovate.Platform.Type,
		Endpoint:      r.Instance.Spec.Renovate.Platform.Endpoint,
		Repositories:  []string{},
	}

	baseConfig, err := json.Marshal(renovateConfig)
	if err != nil {
		return nil, err
	}

	data := map[string]string{
		"renovate.json": string(baseConfig),
	}

	// if batches could be retrieved add them
	if r.Batches != nil {
		batchesString, err := json.Marshal(r.Batches)
		if err != nil {
			return nil, err
		}

		data["batches.json"] = string(batchesString)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      r.Instance.Name,
			Namespace: r.Instance.Namespace,
		},
		Data: data,
	}
	if err := controllerutil.SetControllerReference(r.Instance, cm, r.Scheme); err != nil {
		return nil, err
	}

	return cm, nil
}
