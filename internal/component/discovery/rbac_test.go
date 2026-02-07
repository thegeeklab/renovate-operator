package discovery

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	. "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("RBAC Reconciliation", func() {
	var (
		fakeClient client.Client
		reconciler *Reconciler
		instance   *renovatev1beta1.Discovery
		ctx        context.Context
		scheme     *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			Build()

		instance = &renovatev1beta1.Discovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-discovery",
				Namespace: "test-namespace",
			},
			Spec: renovatev1beta1.DiscoverySpec{},
		}
		dd := &DiscoveryCustomDefaulter{}
		Expect(dd.Default(ctx, instance)).To(Succeed())

		reconciler = &Reconciler{
			Client:   fakeClient,
			scheme:   scheme,
			req:      ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "test-namespace", Name: "test-discovery"}},
			instance: instance,
		}

		ctx = context.Background()
	})

	Context("when reconciling ServiceAccount", func() {
		It("should create or update the service account", func() {
			// Execute
			result, err := reconciler.reconcileServiceAccount(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			// Verify service account was created
			sa := &corev1.ServiceAccount{ObjectMeta: metadata.GenericMetadata(reconciler.req)}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(sa), sa)).To(Succeed())
			Expect(sa.Name).To(Equal(metadata.GenericMetadata(reconciler.req).Name))
			Expect(sa.Namespace).To(Equal(reconciler.req.Namespace))
		})
	})

	Context("when reconciling Role", func() {
		It("should create or update the role with correct rules", func() {
			// Execute
			result, err := reconciler.reconcileRole(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			// Verify role was created
			role := &rbacv1.Role{ObjectMeta: metadata.GenericMetadata(reconciler.req)}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(role), role)).To(Succeed())

			// Verify role rules
			Expect(role.Rules).To(HaveLen(3))

			// Rule 1: Allow reading the renovator instance
			Expect(role.Rules[0].APIGroups).To(ContainElement(renovatev1beta1.GroupVersion.Group))
			Expect(role.Rules[0].Resources).To(ContainElement(renovatev1beta1.ResourceRenovators.String()))
			Expect(role.Rules[0].ResourceNames).To(ContainElement(instance.Name))
			Expect(role.Rules[0].Verbs).To(ContainElement("get"))

			// Rule 2: Allow reading discoveries
			Expect(role.Rules[1].APIGroups).To(ContainElement(renovatev1beta1.GroupVersion.Group))
			Expect(role.Rules[1].Resources).To(ContainElement(renovatev1beta1.ResourceDiscoveries.String()))
			Expect(role.Rules[1].Verbs).To(ContainElements("get", "list"))

			// Rule 3: Allow managing configmaps
			Expect(role.Rules[2].APIGroups).To(ContainElement(corev1.SchemeGroupVersion.Group))
			Expect(role.Rules[2].Resources).To(ContainElement(corev1.ResourceConfigMaps.String()))
			Expect(role.Rules[2].Verbs).To(ContainElements("get", "list", "create", "update", "patch", "delete"))
		})
	})

	Context("when reconciling RoleBinding", func() {
		It("should create or update the role binding with correct subjects and role reference", func() {
			// Execute
			result, err := reconciler.reconcileRoleBinding(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			// Verify role binding was created
			rb := &rbacv1.RoleBinding{ObjectMeta: metadata.GenericMetadata(reconciler.req)}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(rb), rb)).To(Succeed())

			// Verify role binding subjects
			Expect(rb.Subjects).To(HaveLen(1))
			Expect(rb.Subjects[0].Kind).To(Equal("ServiceAccount"))
			Expect(rb.Subjects[0].Name).To(Equal(metadata.GenericMetadata(reconciler.req).Name))
			Expect(rb.Subjects[0].Namespace).To(Equal(reconciler.req.Namespace))

			// Verify role binding role reference
			Expect(rb.RoleRef.APIGroup).To(Equal("rbac.authorization.k8s.io"))
			Expect(rb.RoleRef.Kind).To(Equal("Role"))
			Expect(rb.RoleRef.Name).To(Equal(metadata.GenericMetadata(reconciler.req).Name))
		})
	})

	Context("when updating RoleBinding", func() {
		It("should correctly set subjects and role reference", func() {
			// Create a role binding
			rb := &rbacv1.RoleBinding{ObjectMeta: metadata.GenericMetadata(reconciler.req)}
			Expect(fakeClient.Create(ctx, rb)).To(Succeed())

			// Execute update
			err := reconciler.updateRoleBinding(rb)
			Expect(err).ToNot(HaveOccurred())

			// Verify role binding was updated correctly
			Expect(rb.Subjects).To(HaveLen(1))
			Expect(rb.Subjects[0].Kind).To(Equal("ServiceAccount"))
			Expect(rb.Subjects[0].Name).To(Equal(metadata.GenericMetadata(reconciler.req).Name))
			Expect(rb.Subjects[0].Namespace).To(Equal(reconciler.req.Namespace))

			Expect(rb.RoleRef.APIGroup).To(Equal("rbac.authorization.k8s.io"))
			Expect(rb.RoleRef.Kind).To(Equal("Role"))
			Expect(rb.RoleRef.Name).To(Equal(metadata.GenericMetadata(reconciler.req).Name))
		})
	})

	Context("when updating Role", func() {
		It("should correctly set role rules", func() {
			// Create a role
			role := &rbacv1.Role{ObjectMeta: metadata.GenericMetadata(reconciler.req)}
			Expect(fakeClient.Create(ctx, role)).To(Succeed())

			// Execute update
			err := reconciler.updateRole(role)
			Expect(err).ToNot(HaveOccurred())

			// Verify role rules were set correctly
			Expect(role.Rules).To(HaveLen(3))

			// Rule 1: Allow reading the renovator instance
			Expect(role.Rules[0].APIGroups).To(ContainElement(renovatev1beta1.GroupVersion.Group))
			Expect(role.Rules[0].Resources).To(ContainElement(renovatev1beta1.ResourceRenovators.String()))
			Expect(role.Rules[0].ResourceNames).To(ContainElement(instance.Name))
			Expect(role.Rules[0].Verbs).To(ContainElement("get"))

			// Rule 2: Allow reading discoveries
			Expect(role.Rules[1].APIGroups).To(ContainElement(renovatev1beta1.GroupVersion.Group))
			Expect(role.Rules[1].Resources).To(ContainElement(renovatev1beta1.ResourceDiscoveries.String()))
			Expect(role.Rules[1].Verbs).To(ContainElements("get", "list"))

			// Rule 3: Allow managing configmaps
			Expect(role.Rules[2].APIGroups).To(ContainElement(corev1.SchemeGroupVersion.Group))
			Expect(role.Rules[2].Resources).To(ContainElement(corev1.ResourceConfigMaps.String()))
			Expect(role.Rules[2].Verbs).To(ContainElements("get", "list", "create", "update", "patch", "delete"))
		})
	})
})
