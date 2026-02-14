package discovery

import (
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
