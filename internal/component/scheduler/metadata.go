package scheduler

import (
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// SchedulerGroupName is the group name used for scheduler components.
	SchedulerGroupName = "scheduler"
)

func SchedulerMetadata(request ctrl.Request) v1.ObjectMeta {
	return v1.ObjectMeta{
		Name:      SchedulerName(request),
		Namespace: request.Namespace,
	}
}

func SchedulerName(request ctrl.Request) string {
	return metadata.BuildName(request.Name, SchedulerGroupName)
}
