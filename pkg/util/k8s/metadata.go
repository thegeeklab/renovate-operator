package k8s

import (
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	api_util "sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func trimList(gvk schema.GroupVersionKind) schema.GroupVersionKind {
	gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")

	return gvk
}

func GVK(scheme *runtime.Scheme, obj runtime.Object) schema.GroupVersionKind {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Kind != "" {
		return trimList(gvk)
	}

	gvk, _ = api_util.GVKForObject(obj, scheme)

	return trimList(gvk)
}

func GetNamespace() string {
	ns, found := os.LookupEnv("POD_NAMESPACE")
	if !found {
		return "kube-system"
	}

	return ns
}
