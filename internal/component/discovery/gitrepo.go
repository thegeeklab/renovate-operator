package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileGitRepos synchronizes GitRepo resources based on the discovery result ConfigMap.
func (r *Reconciler) reconcileGitRepos(ctx context.Context) (*ctrl.Result, error) {
	var (
		allErrors []error
		targetCM  *corev1.ConfigMap
	)

	log := logf.FromContext(ctx)
	discoveredRepoMatcher := make(map[string]bool)

	// 1. Find the ConfigMap using the owner reference
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

	// 2. If no ConfigMap found, skip sync
	if targetCM == nil {
		log.V(1).Info("No discovery result ConfigMap found, skipping GitRepo sync")

		return &ctrl.Result{}, nil
	}

	// 3. Validate ConfigMap data
	repoData, exists := targetCM.Data["repositories"]
	if !exists {
		log.Error(nil, "ConfigMap does not contain 'repositories' key", "cm", targetCM.Name)

		return &ctrl.Result{}, nil
	}

	// 4. Parse repositories from the owned ConfigMap
	var discoveredRepos []string
	if err := json.Unmarshal([]byte(repoData), &discoveredRepos); err != nil {
		log.Error(err, "Failed to unmarshal discovery results from ConfigMap", "cm", targetCM.Name)

		return &ctrl.Result{}, nil
	}

	// 5. Sync State (Create/Update)
	for _, repoName := range discoveredRepos {
		discoveredRepoMatcher[repoName] = true

		sanitizedName, err := k8s.SanitizeName(repoName)
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
	}

	// 6. Prune
	if err := r.pruneOrphanedRepos(ctx, discoveredRepoMatcher); err != nil {
		allErrors = append(allErrors, fmt.Errorf("failed to prune orphaned repos: %w", err))
	}

	// Return aggregated errors if any
	if len(allErrors) > 0 {
		return &ctrl.Result{}, errors.Join(allErrors...)
	}

	return &ctrl.Result{}, nil
}

// updateGitRepo manages the specific spec and labels of the GitRepo resource.
func (r *Reconciler) updateGitRepo(gr *renovatev1beta1.GitRepo, repoName string) error {
	if gr.Labels == nil {
		gr.Labels = make(map[string]string)
	}

	if r.instance.Labels != nil {
		if renovator, ok := r.instance.Labels[renovatev1beta1.RenovatorLabel]; ok {
			gr.Labels[renovatev1beta1.RenovatorLabel] = renovator
		}
	}

	gr.Spec.Name = repoName

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
