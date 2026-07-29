package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/provider"
	"github.com/thegeeklab/renovate-operator/internal/provider/factory"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var ErrPlatformTokenSecretNotConfigured = errors.New("platform token secret not configured")

// reconcileGitRepos synchronizes GitRepo resources based on the discovery result ConfigMap.
func (r *Reconciler) reconcileGitRepos(ctx context.Context) (*ctrl.Result, error) {
	var (
		allErrors []error
		targetCM  *corev1.ConfigMap
	)

	log := logf.FromContext(ctx)

	cms := &corev1.ConfigMapList{}
	if err := r.List(ctx, cms, client.InNamespace(r.instance.Namespace)); err != nil {
		return &ctrl.Result{}, err
	}

	for _, cm := range cms.Items {
		if metav1.IsControlledBy(&cm, r.instance) {
			targetCM = &cm

			break
		}
	}

	if targetCM == nil {
		log.V(1).Info("No discovery result ConfigMap found, skipping GitRepo sync")

		return &ctrl.Result{}, nil
	}

	repoData, exists := targetCM.Data["repositories"]
	if !exists {
		log.Error(nil, "ConfigMap does not contain 'repositories' key", "cm", targetCM.Name)

		return &ctrl.Result{}, nil
	}

	var discoveredRepos []string
	if err := json.Unmarshal([]byte(repoData), &discoveredRepos); err != nil {
		log.Error(err, "Failed to unmarshal discovery results from ConfigMap", "cm", targetCM.Name)

		return &ctrl.Result{}, nil
	}

	filteredRepos, err := r.filterRepos(ctx, discoveredRepos)
	if err != nil {
		return &ctrl.Result{}, err
	}

	if r.metrics != nil {
		renovatorLabel := r.instance.Labels[renovatev1beta1.LabelRenovator]
		r.metrics.SetDiscoveryRepositories(r.instance.Namespace, renovatorLabel, r.instance.Name, len(filteredRepos))
	}

	repoMatcher := make(map[string]bool, len(filteredRepos))

	for _, repoName := range filteredRepos {
		sanitizedName, err := k8s.SanitizeSubdomain(repoName)
		if err != nil {
			log.Error(err, "Failed to sanitize repository name", "repo", repoName)
			allErrors = append(allErrors, fmt.Errorf("failed to sanitize repo name %s: %w", repoName, err))

			continue
		}

		gitRepo := &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", r.instance.Name, sanitizedName),
				Namespace: r.instance.Namespace,
			},
		}

		_, err = k8s.CreateOrUpdate(ctx, r.Client, gitRepo, r.instance, func() error {
			return r.updateGitRepo(gitRepo, repoName)
		})
		if err != nil {
			log.Error(err, "Failed to sync GitRepo", "repo", repoName)
			allErrors = append(allErrors, fmt.Errorf("failed to sync GitRepo %s: %w", repoName, err))

			continue
		}

		repoMatcher[repoName] = true
	}

	if err := r.pruneOrphanedRepos(ctx, repoMatcher); err != nil {
		allErrors = append(allErrors, fmt.Errorf("failed to prune orphaned repos: %w", err))
	}

	if len(allErrors) > 0 {
		return &ctrl.Result{}, errors.Join(allErrors...)
	}

	return &ctrl.Result{}, nil
}

// filterRepos returns repos with forks removed when skipForks is enabled and/or
// filtered by topics when topics are configured on the discovery instance.
// The full repository list is fetched in a single batched call per provider, with forks
// excluded server-side (GitHub) or filtered locally (Gitea) to avoid N+1 API calls.
func (r *Reconciler) filterRepos(ctx context.Context, repos []string) ([]string, error) {
	log := logf.FromContext(ctx)

	skipForks := r.instance.GetSkipForks()
	topics := r.instance.GetTopics()
	skipPending := r.instance.GetSkipPendingDeletion()

	if !skipForks && len(topics) == 0 && !skipPending {
		return repos, nil
	}

	if r.renovate.Spec.Platform.Token.SecretKeyRef == nil {
		return nil, ErrPlatformTokenSecretNotConfigured
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      r.renovate.Spec.Platform.Token.SecretKeyRef.Name,
		Namespace: r.instance.Namespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("failed to get platform token secret: %w", err)
	}

	platformConfig := factory.PlatformConfig{
		Type:     string(r.renovate.Spec.Platform.Type),
		Endpoint: r.renovate.Spec.Platform.Endpoint,
		Token:    string(secret.Data[r.renovate.Spec.Platform.Token.SecretKeyRef.Key]),
	}

	providerManager, err := r.providerFactory(ctx, platformConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider: %w", err)
	}

	platformRepos, err := providerManager.ListRepos(ctx, provider.ListReposOptions{
		SkipForks:           skipForks,
		Topics:              topics,
		SkipPendingDeletion: skipPending,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	platformSet := make(map[string]struct{}, len(platformRepos))
	for _, repo := range platformRepos {
		platformSet[repo.Name] = struct{}{}
	}

	filtered := make([]string, 0, len(repos))
	for _, repoName := range repos {
		if _, ok := platformSet[repoName]; !ok {
			log.V(1).Info("Skipping repository excluded by filter", "repo", repoName)

			continue
		}

		filtered = append(filtered, repoName)
	}

	return filtered, nil
}

// updateGitRepo manages the specific spec and labels of the GitRepo resource.
func (r *Reconciler) updateGitRepo(gr *renovatev1beta1.GitRepo, repoName string) error {
	if gr.Labels == nil {
		gr.Labels = make(map[string]string)
	}

	if r.instance.Labels != nil {
		if renovator, ok := r.instance.Labels[renovatev1beta1.LabelRenovator]; ok {
			gr.Labels[renovatev1beta1.LabelRenovator] = renovator
		}
	}

	gr.Spec.Name = repoName
	gr.Spec.Webhooks.Enabled = r.instance.Spec.Webhooks.Enabled

	return nil
}

// pruneOrphanedRepos deletes GitRepos that are no longer present in the discovery result.
func (r *Reconciler) pruneOrphanedRepos(ctx context.Context, discovered map[string]bool) error {
	var pruneErrors []error

	log := logf.FromContext(ctx)

	existingRepos := &renovatev1beta1.GitRepoList{}
	if err := r.List(ctx, existingRepos, client.InNamespace(r.instance.Namespace)); err != nil {
		return fmt.Errorf("failed to list existing GitRepos: %w", err)
	}

	for _, repo := range existingRepos.Items {
		if !discovered[repo.Spec.Name] && metav1.IsControlledBy(&repo, r.instance) {
			log.Info("Deleting orphaned GitRepo", "name", repo.Name)

			if err := r.Delete(ctx, &repo); err != nil {
				log.Error(err, "Failed to delete orphaned GitRepo", "name", repo.Name)
				pruneErrors = append(pruneErrors, fmt.Errorf("failed to delete GitRepo %s: %w", repo.Name, err))
			}
		}
	}

	if len(pruneErrors) > 0 {
		return errors.Join(pruneErrors...)
	}

	return nil
}
