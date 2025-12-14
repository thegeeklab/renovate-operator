package discovery

import (
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateGitRepo(owner client.Object, namespace, repo string) (*renovatev1beta1.GitRepo, error) {
	sanitizedName, err := k8s.SanitizeName(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to sanitize repository name: %w", err)
	}

	gitRepo := &renovatev1beta1.GitRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sanitizedName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(owner, renovatev1beta1.GroupVersion.WithKind("Renovator")),
			},
		},
		Spec: renovatev1beta1.GitRepoSpec{
			Name: repo,
		},
	}

	return gitRepo, nil
}
