package runner

import (
	"context"
	"fmt"

	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) reconcileConfigMap(ctx context.Context) (*ctrl.Result, error) {
	cm := &corev1.ConfigMap{ObjectMeta: v1.ObjectMeta{
		Name:      r.instance.Name,
		Namespace: r.instance.Namespace,
	}}

	_, err := k8s.CreateOrPatch(ctx, r.Client, cm, r.instance, func() error {
		return r.updateConfigMap(cm)
	})
	if err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to create or update config map: %w", err)
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateConfigMap(cm *corev1.ConfigMap) error {
	data := make(map[string]string)

	// Validate and serialize renovate config
	renovateConfig, err := json.Marshal(r.renovateConfig)
	if err != nil {
		return fmt.Errorf("failed to serialize renovate config: %w", err)
	}

	if len(renovateConfig) > 0 {
		data["renovate.json"] = string(renovateConfig)
	}

	// Validate and serialize batches
	batchesData, err := json.Marshal(r.batches)
	if err != nil {
		return fmt.Errorf("failed to serialize batches: %w", err)
	}

	if len(batchesData) > 0 {
		data["batches.json"] = string(batchesData)
	}

	// Set the data in the config map
	cm.Data = data

	return nil
}
