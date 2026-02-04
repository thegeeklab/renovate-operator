package runner

import (
	"context"
	"fmt"

	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
)

const ConfigMapSuffix = "renovate-batch"

func (r *Reconciler) reconcileConfigMap(ctx context.Context) (*ctrl.Result, error) {
	cm := &corev1.ConfigMap{ObjectMeta: metadata.GenericMetadata(r.req, ConfigMapSuffix)}

	_, err := k8s.CreateOrUpdate(ctx, r.Client, cm, r.instance, func() error {
		return r.updateConfigMap(cm)
	})

	return &ctrl.Result{}, err
}

func (r *Reconciler) updateConfigMap(cm *corev1.ConfigMap) error {
	data := make(map[string]string)

	batchesData, err := json.Marshal(r.batches)
	if err != nil {
		return fmt.Errorf("failed to serialize batches: %w", err)
	}

	if len(batchesData) > 0 {
		data[renovate.FilenameBatches] = string(batchesData)
	}

	cm.Data = data

	return nil
}
