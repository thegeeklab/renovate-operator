package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/discovery"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme = runtime.NewScheme()

	ErrReadDiscoveryFile = errors.New("failed to read discovery file")
)

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

	log := logf.FromContext(ctx).
		WithValues("namespace", d.Namespace, "name", d.Name)

	discoveredRepos, err := readDiscoveryFile(d.FilePath)
	if err != nil {
		return err
	}

	log.Info("Repository list", "repositories", discoveredRepos)

	// Get renovator instance as owner ref
	renovator := &renovatev1beta1.Renovator{}

	renovatorName := types.NamespacedName{
		Namespace: d.Namespace,
		Name:      d.Name,
	}

	if err := d.KubeClient.Get(ctx, renovatorName, renovator); err != nil {
		return err
	}

	// Create GitRepo CRs for discovered repos
	discoveredRepoMatcher := make(map[string]bool)

	for _, repo := range discoveredRepos {
		discoveredRepoMatcher[repo] = true

		r, err := discovery.CreateGitRepo(renovator, d.Namespace, repo)
		if err != nil {
			log.Error(err, "Failed to create GitRepo: invalid repository name", "repo", repo)

			continue
		}

		err = d.KubeClient.Create(ctx, r)
		if err != nil && !api_errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create GitRepo", "repo", repo)
		}
	}

	// Clean up removed repos
	existingRepos := &renovatev1beta1.GitRepoList{}
	if err := d.KubeClient.List(ctx, existingRepos, client.InNamespace(d.Namespace)); err != nil {
		return err
	}

	for _, repo := range existingRepos.Items {
		if discoveredRepoMatcher[repo.Spec.Name] || !metav1.IsControlledBy(&repo, renovator) {
			continue
		}

		if err := d.KubeClient.Delete(ctx, &repo); err != nil {
			log.Error(err, "Failed to delete GitRepo", "repo", repo.Name)

			continue
		}

		log.Info("Deleted GitRepo", "repo", repo.Name)
	}

	return nil
}

func readDiscoveryFile(path string) ([]string, error) {
	readBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var repos []string

	if err := json.Unmarshal(readBytes, &repos); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReadDiscoveryFile, err)
	}

	return repos, nil
}
