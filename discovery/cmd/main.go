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

	dc, err := discovery.LoadConfig()
	if err != nil {
		ctxLogger.Error(err, "Failed to get discovery configuration")
		panic(err.Error())
	}

	ctxLogger = ctxLogger.WithValues("namespace", dc.Namespace, "name", dc.Name)

	renovatorName := types.NamespacedName{
		Namespace: dc.Namespace,
		Name:      dc.Name,
	}

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		ctxLogger.Error(err, "Failed to initialize cluster internal kubeconfig")

		if dc.KubeConfigPath == "" {
			ctxLogger.Error(err, "Failed to initialize cluster external kubeconfig")
			panic(err.Error())
		}

		kubeConfig, err = clientcmd.BuildConfigFromFlags("", dc.KubeConfigPath)
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

	di := &renovatev1beta1.Discovery{}

	err = cl.Get(ctx, renovatorName, di)
	if err != nil {
		ctxLogger.Error(err, "Failed to retrieve renovator instance")
		panic(err.Error())
	}

	readBytes, err := os.ReadFile(dc.FilePath)
	if err != nil {
		ctxLogger.Error(err, "Failed to read file", "file", dc.FilePath)
		panic(err.Error())
	}

	repositories := make([]string, 0)

	err = json.Unmarshal(readBytes, repositories)
	if err != nil {
		ctxLogger.Error(err, "Failed to unmarshal json")
		panic(err.Error())
	}

	di.Status.Repositories = repositories

	err = cl.Status().Update(ctx, di)
	if err != nil {
		ctxLogger.Error(err, "Failed to update status of renovator instance")
		panic(err.Error())
	}
}
