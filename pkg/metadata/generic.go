package metadata

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func GenericMetaData(request ctrl.Request) v1.ObjectMeta {
	return v1.ObjectMeta{
		Name:      GenericName(request),
		Namespace: request.Namespace,
	}
}

func GenericName(request ctrl.Request) string {
	return request.Name
}
