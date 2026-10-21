package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/test/utils"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const (
	defaultedRenovatorName = "renovator-defaulted"
	finalizerRenovatorName = "renovator-finalizer-test"
	rejectedRenovatorName  = "renovator-rejected"
	e2eTokenSecretName     = "renovate-token-e2e"
	helmReleaseName        = "renovate-operator-e2e"

	healthEndpoint = "/healthz"
	readyEndpoint  = "/readyz"
)

// TestOperator verifies that the operator Deployment becomes healthy, that the
// certificate controller injects the CA bundle into the webhook configurations,
// that the admission webhook is reachable over TLS and that the validating
// webhook actually rejects invalid resources. The CA bundle check, defaulting
// check and rejection check together prove the API server can complete a real
// TLS handshake with the webhook server in both directions, which envtest
// cannot verify.
func TestOperator(t *testing.T) {
	deploymentFeature := features.New("operator deployment").
		Setup(createTestTokenSecret(e2eTokenSecretName)).
		Assess("manager deployment becomes available", managerDeploymentAvailable).
		Assess("webhook CA bundle is injected", webhookCABundleInjected).
		Assess("health and metrics endpoints are accessible", managerEndpointsHealthy).
		Assess("renovator defaults applied through admission webhook", renovatorDefaultsApplied).
		Assess("renovator validation rejects invalid resource through admission webhook", renovatorValidationRejected).
		Assess("renovator cleanup removes all child resources via garbage collection", renovatorFinalizerCleanup).
		Teardown(deleteTestResources(e2eTokenSecretName, []string{defaultedRenovatorName, finalizerRenovatorName})).
		Feature()

	testEnv.Test(t, deploymentFeature)
}

// TestHelmChart verifies that the operator can be installed, becomes healthy
// and can be uninstalled via Helm. This validates the actual distribution
// mechanism used by end users.
func TestHelmChart(t *testing.T) {
	helmFeature := features.New("helm chart installation").
		Setup(helmUndeployKubectlOperator).
		Assess("helm install succeeds", helmInstall).
		Assess("helm installed operator becomes available", helmDeploymentAvailable).
		Assess("helm uninstall succeeds", helmUninstall).
		Teardown(helmRedeployKubectlOperator).
		Feature()

	testEnv.Test(t, helmFeature)
}

func createTestTokenSecret(secretName string) func(
	ctx context.Context, t *testing.T, cfg *envconf.Config,
) context.Context {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()

		secret := &corev1.Secret{
			Name:       secretName,
			Namespace:  testNamespace,
			StringData: map[string]string{"token": "dummy-token"},
		}

		resources := cfg.Client().Resources(testNamespace)
		if err := resources.Create(ctx, secret); err != nil {
			t.Fatalf("failed to create token secret: %v", err)
		}

		return ctx
	}
}

func deleteTestResources(secretName string, renovatorNames []string) func(
	ctx context.Context, t *testing.T, cfg *envconf.Config,
) context.Context {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()

		resources := cfg.Client().Resources(testNamespace)

		for _, name := range renovatorNames {
			renovator := &renovatev1beta1.Renovator{
				Name:      name,
				Namespace: testNamespace,
			}
			if err := resources.Delete(ctx, renovator); err != nil && !apierrors.IsNotFound(err) {
				t.Errorf("failed to delete renovator %s: %v", name, err)
			}
		}

		secret := &corev1.Secret{
			Name:      secretName,
			Namespace: testNamespace,
		}
		if err := resources.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
			t.Errorf("failed to delete token secret %s: %v", secretName, err)
		}

		return ctx
	}
}

func managerDeploymentAvailable(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	resources := cfg.Client().Resources(utils.OperatorNamespace)

	err := wait.For(
		conditions.New(resources).DeploymentAvailable(utils.ManagerDeploymentName, utils.OperatorNamespace),
		wait.WithTimeout(2*time.Minute),
		wait.WithInterval(2*time.Second),
	)
	if err != nil {
		t.Fatalf("manager deployment did not become available: %v", err)
	}

	return ctx
}

// managerEndpointsHealthy verifies that the manager's health and metrics
// endpoints are accessible. This catches breakage in monitoring and alerting
// infrastructure that would otherwise go unnoticed until production.
func managerEndpointsHealthy(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	// Health endpoints are on port 8081
	healthPort := 18081

	healthPortForwardCmd := exec.CommandContext(
		ctx,
		"kubectl",
		"port-forward",
		"-n", utils.OperatorNamespace,
		fmt.Sprintf("deployment/%s", utils.ManagerDeploymentName),
		fmt.Sprintf("%d:8081", healthPort),
	)

	if err := healthPortForwardCmd.Start(); err != nil {
		t.Fatalf("failed to start health port-forward: %v", err)
	}

	defer func() {
		_ = healthPortForwardCmd.Process.Kill()
	}()

	healthBaseURL := fmt.Sprintf("http://localhost:%d", healthPort)

	// Wait for port-forward to become ready using wait.For pattern
	if err := wait.For(func(ctx context.Context) (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthBaseURL+healthEndpoint, nil)
		if err != nil {
			return false, fmt.Errorf("create request: %w", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false, nil // Not an error, just not ready yet
		}
		defer resp.Body.Close()

		return true, nil
	}, wait.WithTimeout(30*time.Second), wait.WithInterval(time.Second)); err != nil {
		t.Fatalf("health port-forward not ready: %v", err)
	}

	// Test health and ready endpoints
	for endpoint, name := range map[string]string{
		healthEndpoint: "health",
		readyEndpoint:  "ready",
	} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthBaseURL+endpoint, nil)
		if err != nil {
			t.Fatalf("failed to create request for %s endpoint: %v", name, err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to access %s endpoint: %v", name, err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s endpoint returned status %d: %s", name, resp.StatusCode, string(body))
		}
	}

	// Verify metrics service exists and has endpoints
	resources := cfg.Client().Resources(utils.OperatorNamespace)

	metricsService := &corev1.Service{}
	metricsServiceName := "renovate-operator-metrics-service"

	if err := resources.Get(ctx, metricsServiceName, utils.OperatorNamespace, metricsService); err != nil {
		t.Fatalf("failed to get metrics service: %v", err)
	}

	endpointSlices := &discoveryv1.EndpointSliceList{}
	if err := resources.List(ctx, endpointSlices); err != nil {
		t.Fatalf("failed to list endpoint slices: %v", err)
	}

	hasReadyMetricsEndpoints := false

	for i := range endpointSlices.Items {
		slice := &endpointSlices.Items[i]
		if slice.Namespace != utils.OperatorNamespace {
			continue
		}

		if serviceName, ok := slice.Labels["kubernetes.io/service-name"]; !ok || serviceName != metricsServiceName {
			continue
		}

		for _, endpoint := range slice.Endpoints {
			if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
				hasReadyMetricsEndpoints = true

				break
			}
		}

		if hasReadyMetricsEndpoints {
			break
		}
	}

	if !hasReadyMetricsEndpoints {
		t.Fatal("metrics service has no ready endpoints")
	}

	return ctx
}

func webhookCABundleInjected(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	resources := cfg.Client().Resources()

	check := func(ctx context.Context) (bool, error) {
		mutating := &admissionregistrationv1.MutatingWebhookConfiguration{}
		if err := resources.Get(ctx, utils.WebhookConfigurationName, "", mutating); err != nil {
			return false, err
		}

		for _, webhook := range mutating.Webhooks {
			if len(webhook.ClientConfig.CABundle) == 0 {
				return false, nil
			}
		}

		validating := &admissionregistrationv1.ValidatingWebhookConfiguration{}
		if err := resources.Get(ctx, utils.WebhookConfigurationName, "", validating); err != nil {
			return false, err
		}

		for _, webhook := range validating.Webhooks {
			if len(webhook.ClientConfig.CABundle) == 0 {
				return false, nil
			}
		}

		return true, nil
	}

	if err := wait.For(check, wait.WithTimeout(2*time.Minute), wait.WithInterval(2*time.Second)); err != nil {
		t.Fatalf("webhook CA bundle was not injected: %v", err)
	}

	return ctx
}

// renovatorDefaultsApplied creates a Renovator through the API server and
// asserts that the mutating admission webhook applied the expected defaults.
// Because envtest does not run a real admission controller, this is the only
// place where the webhook wiring (WebhookConfiguration, service, TLS) is
// exercised end to end.
func renovatorDefaultsApplied(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	resources := cfg.Client().Resources(testNamespace)

	renovator := minimalRenovator(defaultedRenovatorName)
	suspend := true
	renovator.Spec.Suspend = &suspend

	if err := resources.Create(ctx, renovator); err != nil {
		t.Fatalf("failed to create renovator: %v", err)
	}

	got := &renovatev1beta1.Renovator{}
	if err := resources.Get(ctx, renovator.Name, testNamespace, got); err != nil {
		t.Fatalf("failed to get renovator: %v", err)
	}

	if got.Spec.Schedule != renovatev1beta1.DefaultSchedule {
		t.Errorf("schedule not defaulted: got %q, want %q", got.Spec.Schedule, renovatev1beta1.DefaultSchedule)
	}

	if got.Spec.Image != renovatev1beta1.DefaultOperatorContainerImage {
		t.Errorf("image not defaulted: got %q", got.Spec.Image)
	}

	if got.Spec.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("imagePullPolicy not defaulted: got %q", got.Spec.ImagePullPolicy)
	}

	if got.Spec.Renovate.Image != renovatev1beta1.DefaultRenovateContainerImage {
		t.Errorf("renovate image not defaulted: got %q", got.Spec.Renovate.Image)
	}

	if got.Spec.Webhooks.Enabled == nil || !*got.Spec.Webhooks.Enabled {
		t.Errorf("webhooks.enabled not defaulted to true: got %v", got.Spec.Webhooks.Enabled)
	}

	return ctx
}

// renovatorValidationRejected creates a Renovator with an invalid timezone
// through the API server and asserts that the validating admission webhook
// rejects it. This proves the API server performs a real TLS handshake with the
// webhook and enforces validation rules, which envtest cannot verify.
func renovatorValidationRejected(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	resources := cfg.Client().Resources(testNamespace)

	renovator := minimalRenovator(rejectedRenovatorName)
	renovator.Spec.Timezone = "Invalid/Timezone"

	err := resources.Create(ctx, renovator)
	if err == nil {
		_ = resources.Delete(ctx, renovator)

		t.Fatal("validating webhook did not reject renovator with invalid timezone")
	}

	if !strings.Contains(err.Error(), "invalid timezone") {
		t.Fatalf("expected error message to contain 'invalid timezone', got: %v", err)
	}

	return ctx
}

// renovatorFinalizerCleanup creates a Renovator, verifies that child resources
// are created, deletes the Renovator and then verifies that all child resources
// are garbage collected. This catches the most common operator bug: orphaned
// resources left behind after deletion due to missing or broken owner
// references.
func renovatorFinalizerCleanup(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	resources := cfg.Client().Resources(testNamespace)

	renovator := minimalRenovator(finalizerRenovatorName)
	suspend := true
	renovator.Spec.Suspend = &suspend

	if err := resources.Create(ctx, renovator); err != nil {
		t.Fatalf("failed to create renovator: %v", err)
	}

	if err := waitRenovatorReady(ctx, cfg, finalizerRenovatorName); err != nil {
		t.Fatalf("renovator did not become ready: %v\nmanager logs:\n%s", err, utils.ManagerLogs(ctx))
	}

	for _, obj := range []k8s.Object{
		&renovatev1beta1.RenovateConfig{},
		&renovatev1beta1.Discovery{},
		&renovatev1beta1.Runner{},
	} {
		if err := resources.Get(ctx, finalizerRenovatorName, testNamespace, obj); err != nil {
			t.Fatalf("child resource %T not created: %v", obj, err)
		}
	}

	if err := resources.Delete(ctx, renovator); err != nil {
		t.Fatalf("failed to delete renovator: %v", err)
	}

	for _, obj := range []k8s.Object{
		&renovatev1beta1.RenovateConfig{},
		&renovatev1beta1.Discovery{},
		&renovatev1beta1.Runner{},
	} {
		if err := waitResourceDeleted(ctx, cfg, finalizerRenovatorName, obj); err != nil {
			t.Errorf("child resource %T not garbage collected: %v\nstuck renovate resources:\n%s",
				obj, err, dumpStuckRenovateResources(ctx, cfg))
		}
	}

	return ctx
}

func waitRenovatorReady(ctx context.Context, cfg *envconf.Config, name string) error {
	resources := cfg.Client().Resources(testNamespace)

	var lastRenovator *renovatev1beta1.Renovator

	check := func(ctx context.Context) (bool, error) {
		renovator := &renovatev1beta1.Renovator{}
		if err := resources.Get(ctx, name, testNamespace, renovator); err != nil {
			return false, fmt.Errorf("get renovator %s: %w", name, err)
		}

		lastRenovator = renovator

		condition := renovator.GetCondition(renovatev1beta1.ConditionReady)

		return condition != nil && condition.Status == metav1.ConditionTrue, nil
	}

	err := wait.For(check, wait.WithContext(ctx), wait.WithTimeout(2*time.Minute), wait.WithInterval(2*time.Second))
	if err != nil {
		return fmt.Errorf("renovator %s/%s did not become ready: %w\n%s",
			testNamespace, name, err, renovatorConditions(lastRenovator))
	}

	return nil
}

// renovatorConditions renders the Renovator's status conditions so a failure to
// become ready can be diagnosed without collecting operator logs by hand.
func renovatorConditions(r *renovatev1beta1.Renovator) string {
	if r == nil {
		return "renovator status unknown (resource was never observed)"
	}

	var b strings.Builder

	fmt.Fprintf(&b, "renovator %s/%s (generation %d) status conditions:\n",
		r.Namespace, r.Name, r.Generation)

	if len(r.Status.Conditions) == 0 {
		b.WriteString("  <none> (the controller has not recorded a status yet)\n")
	}

	for _, c := range r.Status.Conditions {
		fmt.Fprintf(&b, "  %s=%s reason=%q message=%q observedGeneration=%d lastTransitionTime=%s\n",
			c.Type, c.Status, c.Reason, c.Message, c.ObservedGeneration, c.LastTransitionTime)
	}

	return strings.TrimSuffix(b.String(), "\n")
}

func waitResourceDeleted(ctx context.Context, cfg *envconf.Config, name string, obj k8s.Object) error {
	resources := cfg.Client().Resources(testNamespace)

	check := func(ctx context.Context) (bool, error) {
		if err := resources.Get(ctx, name, testNamespace, obj); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}

			return false, fmt.Errorf("get resource: %w", err)
		}

		return false, nil
	}

	err := wait.For(check, wait.WithContext(ctx), wait.WithTimeout(30*time.Second), wait.WithInterval(2*time.Second))
	if err != nil {
		return fmt.Errorf("resource %T %s/%s still present: %w (finalizers=%v, deletionTimestamp=%v)",
			obj, testNamespace, name, err, obj.GetFinalizers(), obj.GetDeletionTimestamp())
	}

	return nil
}

// dumpStuckRenovateResources lists renovate custom resources that still hold a
// finalizer or a pending deletion timestamp, which is the usual reason a CRD or
// namespace deletion hangs. The output is appended to failure messages.
func dumpStuckRenovateResources(ctx context.Context, cfg *envconf.Config) string {
	resources := cfg.Client().Resources()

	var b strings.Builder

	appendStuck := func(kind, namespace, name string, finalizers []string, deletion *metav1.Time) {
		if len(finalizers) == 0 && deletion == nil {
			return
		}

		fmt.Fprintf(&b, "  %s %s/%s finalizers=%v deletionTimestamp=%v\n",
			kind, namespace, name, finalizers, deletion)
	}

	renovators := &renovatev1beta1.RenovatorList{}
	if err := resources.List(ctx, renovators); err == nil {
		for i := range renovators.Items {
			item := &renovators.Items[i]
			appendStuck("Renovator", item.Namespace, item.Name, item.Finalizers, item.DeletionTimestamp)
		}
	}

	discoveries := &renovatev1beta1.DiscoveryList{}
	if err := resources.List(ctx, discoveries); err == nil {
		for i := range discoveries.Items {
			item := &discoveries.Items[i]
			appendStuck("Discovery", item.Namespace, item.Name, item.Finalizers, item.DeletionTimestamp)
		}
	}

	runners := &renovatev1beta1.RunnerList{}
	if err := resources.List(ctx, runners); err == nil {
		for i := range runners.Items {
			item := &runners.Items[i]
			appendStuck("Runner", item.Namespace, item.Name, item.Finalizers, item.DeletionTimestamp)
		}
	}

	configs := &renovatev1beta1.RenovateConfigList{}
	if err := resources.List(ctx, configs); err == nil {
		for i := range configs.Items {
			item := &configs.Items[i]
			appendStuck("RenovateConfig", item.Namespace, item.Name, item.Finalizers, item.DeletionTimestamp)
		}
	}

	if b.Len() == 0 {
		return "  <no renovate resources with finalizers or pending deletion>"
	}

	return strings.TrimSuffix(b.String(), "\n")
}

// helmUndeployKubectlOperator removes the kubectl-deployed operator so that
// the Helm chart can install cleanly without resource conflicts.
func helmUndeployKubectlOperator(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	// Wait for all CRDs and resources to be fully deleted before Helm install
	deleteArgs := []string{"delete", "-k", "config/default", "--ignore-not-found=true", "--wait=true", "--timeout=2m"}
	if err := utils.RunCommand(ctx, utils.RepoRoot(), "kubectl", deleteArgs...); err != nil {
		t.Fatalf("failed to undeploy kubectl operator: %v\nremaining resources:\n%s",
			err, dumpStuckRenovateResources(ctx, cfg))
	}

	// CRDs are cluster-scoped and survive namespace deletion, so delete them explicitly
	crdArgs := []string{
		"delete", "crd",
		"-l", "app.kubernetes.io/name=renovate-operator",
		"--ignore-not-found=true",
		"--wait=true",
		"--timeout=2m",
	}
	if err := utils.RunCommand(ctx, utils.RepoRoot(), "kubectl", crdArgs...); err != nil {
		// Fallback: delete by name pattern if label doesn't match
		_ = utils.RunCommand(ctx, utils.RepoRoot(), "kubectl", "delete", "crd",
			"authproviders.renovate.thegeeklab.de",
			"discoveries.renovate.thegeeklab.de",
			"gitrepos.renovate.thegeeklab.de",
			"renovateconfigs.renovate.thegeeklab.de",
			"renovators.renovate.thegeeklab.de",
			"runners.renovate.thegeeklab.de",
			"--ignore-not-found=true", "--wait=true", "--timeout=2m")
	}

	return ctx
}

// helmRedeployKubectlOperator reinstalls the kubectl-deployed operator so that
// the test environment cleanup can proceed normally.
func helmRedeployKubectlOperator(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	if _, err := utils.DeployOperator(projectImage)(ctx, cfg); err != nil {
		t.Fatalf("failed to redeploy kubectl operator: %v", err)
	}

	return ctx
}

func helmInstall(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	chartPath := filepath.Join(utils.RepoRoot(), "dist", "chart")

	imageTag := strings.TrimPrefix(projectImage, "docker.io/thegeeklab/renovate-operator:")

	cmd := exec.CommandContext(
		ctx,
		"helm",
		"install",
		helmReleaseName,
		chartPath,
		"--namespace", "helm-test",
		"--create-namespace",
		"--set", "manager.image.tag="+imageTag,
		"--wait",
		"--timeout", "5m",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helm install failed: %v\n%s", err, string(output))
	}

	return ctx
}

func helmDeploymentAvailable(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	resources := cfg.Client().Resources("helm-test")

	err := wait.For(
		conditions.New(resources).DeploymentAvailable(helmReleaseName+"-controller-manager", "helm-test"),
		wait.WithTimeout(2*time.Minute),
		wait.WithInterval(2*time.Second),
	)
	if err != nil {
		t.Fatalf("helm installed deployment did not become available: %v", err)
	}

	return ctx
}

func helmUninstall(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	cmd := exec.CommandContext(
		ctx,
		"helm",
		"uninstall",
		helmReleaseName,
		"--namespace", "helm-test",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helm uninstall failed: %v\n%s", err, string(output))
	}

	deleteCmd := exec.CommandContext(ctx, "kubectl", "delete", "namespace", "helm-test", "--ignore-not-found=true")
	if output, err := deleteCmd.CombinedOutput(); err != nil {
		t.Errorf("failed to delete helm-test namespace: %v\n%s", err, string(output))
	}

	return ctx
}

// minimalRenovator returns a Renovator with only the required fields set so
// that the admission webhook is responsible for filling in the defaults.
func minimalRenovator(name string) *renovatev1beta1.Renovator {
	return &renovatev1beta1.Renovator{
		Name:      name,
		Namespace: testNamespace,
		Spec: renovatev1beta1.RenovatorSpec{
			Discovery: renovatev1beta1.DiscoverySpec{},
			Runner:    renovatev1beta1.RunnerSpec{},
			Renovate: renovatev1beta1.RenovateConfigSpec{
				Platform: renovatev1beta1.PlatformSpec{
					Type:     renovatev1beta1.PlatformType_GITEA,
					Endpoint: "https://gitea.invalid/api/v1",
					Token: corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{Name: e2eTokenSecretName, Key: "token"},
					},
				},
			},
		},
	}
}
