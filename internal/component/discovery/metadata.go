package discovery

import (
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	DiscoveryGroupName = "discovery"
)

func DiscoveryMetadata(request ctrl.Request) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      DiscoveryName(request),
		Namespace: request.Namespace,
	}
}

func DiscoveryName(request ctrl.Request) string {
	return metadata.BuildName(request.Name, DiscoveryGroupName)
}

// DiscoveryLabels returns the standard base labels for discovery resources.
func DiscoveryLabels(request ctrl.Request) map[string]string {
	return map[string]string{
		renovatev1beta1.LabelAppName:      renovatev1beta1.OperatorName,
		renovatev1beta1.LabelAppInstance:  request.Name,
		renovatev1beta1.LabelAppComponent: renovatev1beta1.ComponentDiscovery,
		renovatev1beta1.LabelAppManagedBy: renovatev1beta1.OperatorManagedBy,
	}
}
