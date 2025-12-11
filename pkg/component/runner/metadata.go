package runner

import (
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// RunnerGroupName is the group name used for runner components.
	RunnerGroupName = "runner"
)

func RunnerMetadata(request ctrl.Request) v1.ObjectMeta {
	return v1.ObjectMeta{
		Name:      RunnerName(request),
		Namespace: request.Namespace,
	}
}

func RunnerName(request ctrl.Request) string {
	return metadata.BuildName(request.Name, RunnerGroupName)
}
