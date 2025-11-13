package discovery

import (
	"github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	DiscoveryGroupName = "discovery"
)

func DiscoveryMetaData(request ctrl.Request) v1.ObjectMeta {
	return v1.ObjectMeta{
		Name:      DiscoveryName(request),
		Namespace: request.Namespace,
	}
}

func DiscoveryName(request ctrl.Request) string {
	return metadata.BuildName(request.Name, DiscoveryGroupName)
}

func DiscoveryJobLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/component": DiscoveryGroupName,
		v1beta1.JobTypeLabelKey:       v1beta1.JobTypeLabelValue,
	}
}
