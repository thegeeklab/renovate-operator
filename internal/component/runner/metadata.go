package runner

import (
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// RunnerGroupName is the group name used for runner components.
	RunnerGroupName = "runner"
)

func RunnerMetadata(request ctrl.Request) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      RunnerName(request),
		Namespace: request.Namespace,
	}
}

func RunnerName(request ctrl.Request) string {
	return metadata.BuildName(request.Name, RunnerGroupName)
}

// RunnerLabels returns the standard base labels for discovery resources.
func RunnerLabels(request ctrl.Request) map[string]string {
	return map[string]string{
		renovatev1beta1.LabelAppName:      renovatev1beta1.OperatorName,
		renovatev1beta1.LabelAppInstance:  request.Name,
		renovatev1beta1.LabelAppComponent: renovatev1beta1.ComponentRunner,
		renovatev1beta1.LabelAppManagedBy: renovatev1beta1.OperatorManagedBy,
	}
}
