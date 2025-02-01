package discovery

import (
	"strings"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateGitRepo(owner client.Object, namespace, repo string) *renovatev1beta1.GitRepo {
	gitRepo := &renovatev1beta1.GitRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sanitizeRepoName(repo),
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(owner, renovatev1beta1.GroupVersion.WithKind("Renovator")),
			},
		},
		Spec: renovatev1beta1.GitRepoSpec{
			Name: repo,
		},
	}

	return gitRepo
}

func sanitizeRepoName(repo string) string {
	sanitized := strings.ToLower(strings.ReplaceAll(repo, "/", "-"))

	return "repo-" + sanitized
}
