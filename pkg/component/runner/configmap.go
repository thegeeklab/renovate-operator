package runner

import (
	"context"

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
		return &ctrl.Result{}, err
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateConfigMap(cm *corev1.ConfigMap) error {
	data := make(map[string]string, 0)

	renovate, err := json.Marshal(r.renovateConfig)
	if err != nil {
		return err
	}

	data["renovate.json"] = string(renovate)

	batches, err := json.Marshal(r.batches)
	if err != nil {
		return err
	}

	data["batches.json"] = string(batches)

	cm.Data = data

	return nil
}
