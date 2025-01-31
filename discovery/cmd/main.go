package main

import (
	"context"
	"os"
	"strings"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/discovery"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	ctx := context.Background()
	ctxLogger := logf.FromContext(ctx)

	ctxLogger.Info("Starting discovery process")

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

	var discoveredRepos []string

	err = json.Unmarshal(readBytes, &discoveredRepos)
	if err != nil {
		ctxLogger.Error(err, "Failed to unmarshal json")
		panic(err.Error())
	}

	ctxLogger.Info("Repository list", "repositories", discoveredRepos)

	// Create GitRepo CRs for discovered repos
	discoveredRepoMatcher := make(map[string]bool, 0)
	for _, repo := range discoveredRepos {
		discoveredRepoMatcher[repo] = true

		gitRepo := &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sanitizeRepoName(repo),
				Namespace: d.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(instance, renovatev1beta1.GroupVersion.WithKind("Renovator")),
				},
			},
			Spec: renovatev1beta1.GitRepoSpec{
				Name: repo,
			},
		}

		err = cl.Create(ctx, gitRepo)
		if err != nil && !errors.IsAlreadyExists(err) {
			ctxLogger.Error(err, "Failed to create GitRepo", "repo", repo)
		}
	}

	// Clean up removed repos
	existingRepos := &renovatev1beta1.GitRepoList{}
	if err := cl.List(ctx, existingRepos, client.InNamespace(d.Namespace)); err != nil {
		ctxLogger.Error(err, "Failed to list GitRepos")
		panic(err.Error())
	}

	for _, repo := range existingRepos.Items {
		if discoveredRepoMatcher[repo.Spec.Name] || !metav1.IsControlledBy(&repo, instance) {
			continue
		}

		if err := cl.Delete(ctx, &repo); err != nil {
			ctxLogger.Error(err, "Failed to delete GitRepo", "repo", repo.Name)

			continue
		}

		ctxLogger.Info("Deleted GitRepo", "repo", repo.Name)
	}
}

func sanitizeRepoName(repo string) string {
	sanitized := strings.ToLower(strings.ReplaceAll(repo, "/", "-"))

	return "repo-" + sanitized
}
