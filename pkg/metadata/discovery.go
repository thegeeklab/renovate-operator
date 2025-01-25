package metadata

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func DiscoveryMetaData(request ctrl.Request) v1.ObjectMeta {
	return v1.ObjectMeta{
		Name:      DiscoveryName(request),
		Namespace: request.Namespace,
	}
}

func DiscoveryName(request ctrl.Request) string {
	return buildName(request.Name, discoveryGroupName)
}
