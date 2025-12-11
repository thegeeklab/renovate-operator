package renovator

import (
	"context"
	"fmt"

	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) reconcileConfigMap(ctx context.Context) (*ctrl.Result, error) {
	cm := &corev1.ConfigMap{ObjectMeta: metadata.GenericMetadata(r.req, "renovate-conf")}

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

	// Set the data in the config map
	cm.Data = data

	return nil
}
