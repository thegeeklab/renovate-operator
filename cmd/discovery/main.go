package main

import (
	"context"
	"fmt"
	"os"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(renovatev1beta1.AddToScheme(scheme))
}

func main() {
	logf.SetLogger(zap.New(zap.JSONEncoder()))

	if err := run(context.Background()); err != nil {
		logf.Log.Error(err, "Failed to run discovery")
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	d, err := discovery.New(scheme)
	if err != nil {
		return err
	}

	repos, err := os.ReadFile(d.FilePath)
	if err != nil {
		return err
	}

	discovery := &renovatev1beta1.Discovery{}
	if err := d.KubeClient.Get(ctx, types.NamespacedName{Name: d.Name, Namespace: d.Namespace}, discovery); err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-discovery", d.Name),
			Namespace: d.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, d.KubeClient, cm, func() error {
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}

		cm.Data["repositories"] = string(repos)

		return controllerutil.SetControllerReference(discovery, cm, scheme)
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile configmap: %w", err)
	}

	logf.Log.Info("ConfigMap reconciled", "operation", op, "name", cm.Name)

	return nil
}
