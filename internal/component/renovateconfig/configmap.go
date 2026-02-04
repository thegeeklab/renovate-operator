package renovateconfig

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
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

func (r *Reconciler) reconcileConfigMap(ctx context.Context) (*ctrl.Result, error) {
	cm := &corev1.ConfigMap{ObjectMeta: metadata.GenericMetadata(r.req, ConfigMapSuffix)}

	_, err := k8s.CreateOrUpdate(ctx, r.Client, cm, r.instance, func() error {
		return r.updateConfigMap(cm)
	})

	return &ctrl.Result{}, err
}

func (r *Reconciler) updateConfigMap(cm *corev1.ConfigMap) error {
	data := make(map[string]string)

	renovateConfig := &RenovateConfig{
		Onboarding:    r.instance.Spec.Onboarding != nil && *r.instance.Spec.Onboarding,
		PrHourlyLimit: r.instance.Spec.PrHourlyLimit,
		DryRun:        r.instance.Spec.DryRun,
		Platform:      r.instance.Spec.Platform.Type,
		Endpoint:      r.instance.Spec.Platform.Endpoint,
		AddLabels:     r.instance.Spec.AddLabels,
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
