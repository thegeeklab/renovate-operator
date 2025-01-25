package worker

import (
	"context"

	"github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Renovate struct {
	Onboarding    bool                  `json:"onboarding"`
	PrHourlyLimit int                   `json:"prHourlyLimit"`
	DryRun        bool                  `json:"dryRun"`
	Platform      v1beta1.PlatformTypes `json:"platform"`
	Endpoint      string                `json:"endpoint"`
	AddLabels     []string              `json:"addLabels"`
}

func (w *Worker) reconcileConfig(ctx context.Context) (*ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)
	// coordination config map
	currentCCM := &corev1.ConfigMap{}

	expectedCCM, creationErr := w.createConfigMap()
	if creationErr != nil {
		return nil, creationErr
	}

	// ensure that the CCM exists
	err := w.client.Get(ctx, w.req.NamespacedName, currentCCM)
	if err != nil {
		if errors.IsNotFound(err) {
			if err = w.client.Create(ctx, expectedCCM); err != nil {
				ctxLogger.Error(err, "Failed to create ControlConfigMap")

				return nil, err
			}

			ctxLogger.Info("Created ControlConfigMap")

			return &ctrl.Result{Requeue: true}, nil
		}

		return nil, err
	}

	// update CCM if necessary
	if !equality.Semantic.DeepDerivative(expectedCCM.Data, currentCCM.Data) {
		ctxLogger.Info("Updating base config")

		err := w.client.Update(ctx, expectedCCM)
		if err != nil {
			ctxLogger.Error(err, "Failed to update base config")

			return &ctrl.Result{}, err
		}

		ctxLogger.Info("Updated base config")

		return &ctrl.Result{Requeue: true}, nil
	}

	return &ctrl.Result{}, nil
}

func (w *Worker) createConfigMap() (*corev1.ConfigMap, error) {
	renovateConfig := &Renovate{
		DryRun:        *w.instance.Spec.Renovate.DryRun,
		Onboarding:    *w.instance.Spec.Renovate.Onboarding,
		PrHourlyLimit: w.instance.Spec.Renovate.PrHourlyLimit,
		AddLabels:     w.instance.Spec.Renovate.AddLabels,
		Platform:      w.instance.Spec.Renovate.Platform.Type,
		Endpoint:      w.instance.Spec.Renovate.Platform.Endpoint,
	}

	baseConfig, err := json.Marshal(renovateConfig)
	if err != nil {
		return nil, err
	}

	data := map[string]string{
		"renovate.json": string(baseConfig),
	}

	// if batches could be retrieved add them
	if w.Batches != nil {
		batchesString, err := json.Marshal(w.Batches)
		if err != nil {
			return nil, err
		}

		data["batches"] = string(batchesString)
	}

	newConfigMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      w.instance.Name,
			Namespace: w.instance.Namespace,
		},
		Data: data,
	}
	if err := controllerutil.SetControllerReference(w.instance, newConfigMap, w.scheme); err != nil {
		return nil, err
	}

	return newConfigMap, nil
}
