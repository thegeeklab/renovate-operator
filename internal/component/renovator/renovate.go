package renovator

import (
	"context"
	"encoding/json"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const ConfigMapSuffix = "renovate-conf"

type RenovateConfig struct {
	Onboarding    bool                         `json:"onboarding"`
	PrHourlyLimit int                          `json:"prHourlyLimit"`
	DryRun        renovatev1beta1.DryRun       `json:"dryRun"`
	Platform      renovatev1beta1.PlatformType `json:"platform"`
	Endpoint      string                       `json:"endpoint"`
	AddLabels     []string                     `json:"addLabels,omitempty"`
}

func (r *Reconciler) reconcileRenovateConfig(ctx context.Context) (*ctrl.Result, error) {
	renovate := &renovatev1beta1.RenovateConfig{ObjectMeta: metadata.GenericMetadata(r.req)}

	_, err := k8s.CreateOrUpdate(ctx, r.Client, renovate, r.instance, func() error {
		return r.updateRenovateConfig(renovate)
	})

	return &ctrl.Result{}, err
}

func (r *Reconciler) updateRenovateConfig(renovate *renovatev1beta1.RenovateConfig) error {
	renovate.Spec = r.instance.Spec.Renovate

	if renovate.Spec.Logging == nil {
		renovate.Spec.Logging = &r.instance.Spec.Logging
	}

	return nil
}

func (r *Reconciler) reconcileRenovateConfigMap(ctx context.Context) (*ctrl.Result, error) {
	cm := &corev1.ConfigMap{ObjectMeta: metadata.GenericMetadata(r.req, ConfigMapSuffix)}

	_, err := k8s.CreateOrUpdate(ctx, r.Client, cm, r.instance, func() error {
		return r.updateConfigMap(cm)
	})

	return &ctrl.Result{}, err
}

func (r *Reconciler) updateConfigMap(cm *corev1.ConfigMap) error {
	data := make(map[string]string)

	renovateConfig := &RenovateConfig{
		Onboarding:    r.instance.Spec.Renovate.Onboarding != nil && *r.instance.Spec.Renovate.Onboarding,
		PrHourlyLimit: r.instance.Spec.Renovate.PrHourlyLimit,
		DryRun:        r.instance.Spec.Renovate.DryRun,
		Platform:      r.instance.Spec.Renovate.Platform.Type,
		Endpoint:      r.instance.Spec.Renovate.Platform.Endpoint,
		AddLabels:     r.instance.Spec.Renovate.AddLabels,
	}

	rc, err := json.Marshal(renovateConfig)
	if err != nil {
		return fmt.Errorf("failed to serialize renovate config: %w", err)
	}

	if len(rc) > 0 {
		data[renovate.FilenameRenovateConfig] = string(rc)
	}

	cm.Data = data

	return nil
}
