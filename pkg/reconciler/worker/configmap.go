package worker

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/equality"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Renovate struct {
	Onboarding    bool                          `json:"onboarding"`
	PrHourlyLimit int                           `json:"prHourlyLimit"`
	DryRun        bool                          `json:"dryRun"`
	Platform      renovatev1beta1.PlatformTypes `json:"platform"`
	Endpoint      string                        `json:"endpoint"`
	AddLabels     []string                      `json:"addLabels,omitempty"`
}

func (r *workerReconciler) reconcileConfigMap(ctx context.Context) (*ctrl.Result, error) {
	expected, err := r.createConfigMap()
	if err != nil {
		return &ctrl.Result{}, err
	}

	return r.ReconcileResource(ctx, &corev1.ConfigMap{}, expected, equality.ConfigMapEqual)
}

func (r *workerReconciler) createConfigMap() (*corev1.ConfigMap, error) {
	renovateConfig := &Renovate{
		DryRun:        *r.instance.Spec.Renovate.DryRun,
		Onboarding:    *r.instance.Spec.Renovate.Onboarding,
		PrHourlyLimit: r.instance.Spec.Renovate.PrHourlyLimit,
		AddLabels:     r.instance.Spec.Renovate.AddLabels,
		Platform:      r.instance.Spec.Renovate.Platform.Type,
		Endpoint:      r.instance.Spec.Renovate.Platform.Endpoint,
	}

	baseConfig, err := json.Marshal(renovateConfig)
	if err != nil {
		return nil, err
	}

	data := map[string]string{
		"renovate.json": string(baseConfig),
	}

	// if batches could be retrieved add them
	if r.batches != nil {
		batchesString, err := json.Marshal(r.batches)
		if err != nil {
			return nil, err
		}

		data["batches"] = string(batchesString)
	}

	newConfigMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      r.instance.Name,
			Namespace: r.instance.Namespace,
		},
		Data: data,
	}
	if err := controllerutil.SetControllerReference(r.instance, newConfigMap, r.Scheme); err != nil {
		return nil, err
	}

	return newConfigMap, nil
}
