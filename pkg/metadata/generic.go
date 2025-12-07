package metadata

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func GenericMetadata(request ctrl.Request, suffix ...string) v1.ObjectMeta {
	return v1.ObjectMeta{
		Name:      GenericName(request, suffix...),
		Namespace: request.Namespace,
	}
}

func GenericName(request ctrl.Request, suffix ...string) string {
	name := request.Name
	if len(suffix) > 0 && suffix[0] != "" {
		name += "-" + suffix[0]
	}

	return name
}
