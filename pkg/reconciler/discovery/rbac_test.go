package discovery

import (
	"testing"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/reconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateRole(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	_ = renovatev1beta1.AddToScheme(scheme)

	renovator := &renovatev1beta1.Renovator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-renovator",
			Namespace: "default",
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(renovator).
		Build()

	r := &discoveryReconciler{
		GenericReconciler: &reconciler.GenericReconciler{
			KubeClient: client,
			Scheme:     scheme,
			Req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-renovator",
					Namespace: "default",
				},
			},
		},
		instance: renovator,
	}

	// Execute
	role, err := r.createRole()
	if err != nil {
		t.Fatalf("createRole() error = %v", err)
	}

	// Verify
	if role.Name != "test-renovator" {
		t.Errorf("Expected role name 'test-renovator', got '%s'", role.Name)
	}

	if role.Namespace != "default" {
		t.Errorf("Expected role namespace 'default', got '%s'", role.Namespace)
	}

	// Check that all required permissions are present
	expectedPermissions := map[string][]string{
		"renovators":    {"get"},
		"gitrepos":      {"get", "list", "create", "update", "patch", "delete"},
		"renovatorjobs": {"get", "list", "create", "update", "patch", "delete"},
	}

	if len(role.Rules) != len(expectedPermissions) {
		t.Errorf("Expected %d rules, got %d", len(expectedPermissions), len(role.Rules))
	}

	for _, rule := range role.Rules {
		if len(rule.APIGroups) != 1 || rule.APIGroups[0] != renovatev1beta1.GroupVersion.Group {
			t.Errorf("Expected API group '%s', got %v", renovatev1beta1.GroupVersion.Group, rule.APIGroups)
		}

		if len(rule.Resources) != 1 {
			t.Errorf("Expected 1 resource per rule, got %d", len(rule.Resources))
			continue
		}

		resource := rule.Resources[0]
		expectedVerbs, exists := expectedPermissions[resource]
		if !exists {
			t.Errorf("Unexpected resource '%s' in role", resource)
			continue
		}

		if len(rule.Verbs) != len(expectedVerbs) {
			t.Errorf("Resource '%s': expected %d verbs, got %d", resource, len(expectedVerbs), len(rule.Verbs))
			continue
		}

		for _, expectedVerb := range expectedVerbs {
			found := false
			for _, verb := range rule.Verbs {
				if verb == expectedVerb {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Resource '%s': missing verb '%s'", resource, expectedVerb)
			}
		}
	}
}

func TestCreateRoleBinding(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	_ = renovatev1beta1.AddToScheme(scheme)

	renovator := &renovatev1beta1.Renovator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-renovator",
			Namespace: "default",
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(renovator).
		Build()

	r := &discoveryReconciler{
		GenericReconciler: &reconciler.GenericReconciler{
			KubeClient: client,
			Scheme:     scheme,
			Req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-renovator",
					Namespace: "default",
				},
			},
		},
		instance: renovator,
	}

	// Execute
	roleBinding, err := r.createRoleBinding()
	if err != nil {
		t.Fatalf("createRoleBinding() error = %v", err)
	}

	// Verify
	if roleBinding.Name != "test-renovator" {
		t.Errorf("Expected role binding name 'test-renovator', got '%s'", roleBinding.Name)
	}

	if roleBinding.Namespace != "default" {
		t.Errorf("Expected role binding namespace 'default', got '%s'", roleBinding.Namespace)
	}

	if len(roleBinding.Subjects) != 1 {
		t.Errorf("Expected 1 subject, got %d", len(roleBinding.Subjects))
	} else {
		subject := roleBinding.Subjects[0]
		if subject.Kind != "ServiceAccount" {
			t.Errorf("Expected subject kind 'ServiceAccount', got '%s'", subject.Kind)
		}
		if subject.Name != "test-renovator" {
			t.Errorf("Expected subject name 'test-renovator', got '%s'", subject.Name)
		}
		if subject.Namespace != "default" {
			t.Errorf("Expected subject namespace 'default', got '%s'", subject.Namespace)
		}
	}

	if roleBinding.RoleRef.APIGroup != "rbac.authorization.k8s.io" {
		t.Errorf("Expected role ref API group 'rbac.authorization.k8s.io', got '%s'", roleBinding.RoleRef.APIGroup)
	}
	if roleBinding.RoleRef.Kind != "Role" {
		t.Errorf("Expected role ref kind 'Role', got '%s'", roleBinding.RoleRef.Kind)
	}
	if roleBinding.RoleRef.Name != "test-renovator" {
		t.Errorf("Expected role ref name 'test-renovator', got '%s'", roleBinding.RoleRef.Name)
	}
}
