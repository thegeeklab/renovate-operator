package metadata

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func RunnerMetaData(request ctrl.Request) v1.ObjectMeta {
	return v1.ObjectMeta{
		Name:      RunnerName(request),
		Namespace: request.Namespace,
	}
}

func RunnerName(request ctrl.Request) string {
	return buildName(request.Name, runnerGroupName)
}
