package k8s

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
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

	gvk, _ = apiutil.GVKForObject(obj, scheme)

	return trimList(gvk)
}
