package main

import (
	"context"
	"os"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/discovery"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(renovatev1beta1.AddToScheme(scheme))
}

func main() {
	ctx := context.Background()
	ctxLogger := logf.FromContext(ctx)

	d, err := discovery.LoadConfig()
	if err != nil {
		ctxLogger.Error(err, "Failed to get discovery configuration")
		panic(err.Error())
	}

	ctxLogger = ctxLogger.WithValues("namespace", d.Namespace, "name", d.Name)

	renovatorName := types.NamespacedName{
		Namespace: d.Namespace,
		Name:      d.Name,
	}

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		ctxLogger.Error(err, "Failed to initialize cluster internal kubeconfig")

		if d.KubeConfigPath == "" {
			ctxLogger.Error(err, "Failed to initialize cluster external kubeconfig")
			panic(err.Error())
		}

		kubeConfig, err = clientcmd.BuildConfigFromFlags("", d.KubeConfigPath)
		if err != nil {
			ctxLogger.Error(err, "Failed to read defined kubeconfig")
			panic(err.Error())
		}
	}

	cl, err := client.New(kubeConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		ctxLogger.Error(err, "Failed to create Kubernetes client")
		panic(err.Error())
	}

	instance := &renovatev1beta1.Renovator{}

	err = cl.Get(ctx, renovatorName, instance)
	if err != nil {
		ctxLogger.Error(err, "Failed to retrieve renovator instance")
		panic(err.Error())
	}

	readBytes, err := os.ReadFile(d.FilePath)
	if err != nil {
		ctxLogger.Error(err, "Failed to read file", "file", d.FilePath)
		panic(err.Error())
	}

	repositories := make([]string, 0)

	err = json.Unmarshal(readBytes, repositories)
	if err != nil {
		ctxLogger.Error(err, "Failed to unmarshal json")
		panic(err.Error())
	}

	instance.Status.Repositories = repositories

	err = cl.Status().Update(ctx, instance)
	if err != nil {
		ctxLogger.Error(err, "Failed to update status of renovator instance")
		panic(err.Error())
	}
}
