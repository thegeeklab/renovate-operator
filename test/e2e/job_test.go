package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/test/utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const (
	jobExecutionRenovatorName = "renovator-job-execution"
	jobExecutionTokenSecret   = "renovate-token-job-execution"

	// discoveredRepoA and discoveredRepoB must stay in sync with the fixture
	// written by the stub image (test/e2e/fixtures/stub/entrypoint.sh).
	discoveredRepoA = "testorg/repo-a"
	discoveredRepoB = "testorg/repo-b"
)

// conditionObject is a Kubernetes object that can report its status conditions.
type conditionObject interface {
	k8s.Object
	GetCondition(string) *metav1.Condition
}

// TestJobExecution verifies the operator's job-execution flow end to end:
// a manual discovery run produces GitRepo resources from the discovery result,
// and a manual renovate run creates a Job that completes and is reflected in
// the GitRepo status. The Renovate container image is replaced with a no-op
// stub so the flow is exercised without pulling the real Renovate image or
// contacting a Git provider, which envtest cannot cover because it needs the
// real kube-scheduler and job controller.
func TestJobExecution(t *testing.T) {
	jobFeature := features.New("job execution").
		Setup(createTestTokenSecret(jobExecutionTokenSecret)).
		Assess("manager deployment becomes available", managerDeploymentAvailable).
		Assess("webhook CA bundle is injected", webhookCABundleInjected).
		Assess("renovator is created", createJobExecutionRenovator).
		Assess("discovery job completes and creates GitRepos", discoveryCreatesGitRepos).
		Assess("renovate job completes and is reflected in status", renovateJobCompletes).
		Teardown(deleteJobExecutionRenovator).
		Teardown(deleteTestResources(jobExecutionTokenSecret, []string{jobExecutionRenovatorName})).
		Feature()

	testEnv.Test(t, jobFeature)
}

func createJobExecutionRenovator(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	resources := cfg.Client().Resources(testNamespace)

	renovator := minimalRenovator(jobExecutionRenovatorName)
	renovator.Spec.Renovate.Image = stubImage
	renovator.Spec.Discovery.Image = projectImage
	renovator.Spec.Renovate.Platform.Token.SecretKeyRef.Name = jobExecutionTokenSecret

	suspend := true
	renovator.Spec.Suspend = &suspend

	webhooksDisabled := false
	renovator.Spec.Webhooks.Enabled = &webhooksDisabled

	// With controller-runtime's cert rotation, webhook certificates are injected
	// asynchronously after the pod becomes ready. The webhook server isn't
	// reachable during this brief window. The retry handles this expected race.
	err := retry.OnError(
		apiwait.Backoff{Duration: time.Second, Factor: 2, Jitter: 0.1, Steps: 7},
		apierrors.IsInternalError,
		func() error { return resources.Create(ctx, renovator) },
	)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("failed to create renovator: %v\n%s", err, utils.ManagerLogs(ctx))
	}

	if err := waitRenovatorReady(ctx, cfg, jobExecutionRenovatorName); err != nil {
		t.Fatalf("renovator did not become ready: %v\n%s", err, utils.ManagerLogs(ctx))
	}

	return ctx
}

func deleteJobExecutionRenovator(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	resources := cfg.Client().Resources(testNamespace)

	renovator := &renovatev1beta1.Renovator{
		Name:      jobExecutionRenovatorName,
		Namespace: testNamespace,
	}
	if err := resources.Delete(ctx, renovator); err != nil && !apierrors.IsNotFound(err) {
		t.Errorf("failed to delete renovator %s: %v", jobExecutionRenovatorName, err)
	}

	return ctx
}

func discoveryCreatesGitRepos(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	triggerRenovatorOperation(ctx, t, cfg, jobExecutionRenovatorName, renovatev1beta1.OperationDiscover)

	discovery := &renovatev1beta1.Discovery{}
	if err := waitForResourceCondition(
		ctx, cfg, jobExecutionRenovatorName, discovery, renovatev1beta1.DiscoveryConditionDiscoveryCompleted,
	); err != nil {
		t.Fatalf("discovery job did not complete: %v\n%s", err, utils.ManagerLogs(ctx))
	}

	resources := cfg.Client().Resources(testNamespace)

	if err := wait.For(func(ctx context.Context) (bool, error) {
		list := &renovatev1beta1.GitRepoList{}
		if err := resources.List(ctx, list); err != nil {
			return false, err
		}

		found := make(map[string]bool, len(list.Items))
		for _, repo := range list.Items {
			found[repo.Spec.Name] = true
		}

		return found[discoveredRepoA] && found[discoveredRepoB], nil
	}, wait.WithTimeout(2*time.Minute), wait.WithInterval(2*time.Second)); err != nil {
		t.Fatalf("discovered GitRepos were not created: %v\n%s", err, utils.ManagerLogs(ctx))
	}

	return ctx
}

func renovateJobCompletes(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()

	triggerRenovatorOperation(ctx, t, cfg, jobExecutionRenovatorName, renovatev1beta1.OperationRenovate)

	gitRepoName, err := gitRepoNameBySpecName(ctx, cfg, discoveredRepoA)
	if err != nil {
		t.Fatalf("failed to find GitRepo %q: %v", discoveredRepoA, err)
	}

	gitRepo := &renovatev1beta1.GitRepo{}
	if err := waitForResourceCondition(
		ctx, cfg, gitRepoName, gitRepo, renovatev1beta1.GitRepoConditionRenovateCompleted,
	); err != nil {
		t.Fatalf("renovate job did not complete: %v\n%s", err, utils.ManagerLogs(ctx))
	}

	return ctx
}

// triggerRenovatorOperation annotates the Renovator with the given operation,
// which the Renovator reconciler propagates to the child Discovery or Runner to
// trigger an immediate run.
func triggerRenovatorOperation(ctx context.Context, t *testing.T, cfg *envconf.Config, name, operation string) {
	t.Helper()

	c := cfg.Client().Resources(testNamespace).GetControllerRuntimeClient()

	renovator := &renovatev1beta1.Renovator{}
	if err := c.Get(ctx, crclient.ObjectKey{Name: name, Namespace: testNamespace}, renovator); err != nil {
		t.Fatalf("failed to get renovator: %v", err)
	}

	patch := crclient.MergeFrom(renovator.DeepCopy())
	if renovator.Annotations == nil {
		renovator.Annotations = make(map[string]string)
	}

	renovator.Annotations[renovatev1beta1.RenovatorOperation] = operation

	if err := c.Patch(ctx, renovator, patch); err != nil {
		t.Fatalf("failed to trigger %q operation: %v", operation, err)
	}
}

// gitRepoNameBySpecName returns the object name of the GitRepo whose spec.name
// matches repoName.
func gitRepoNameBySpecName(ctx context.Context, cfg *envconf.Config, repoName string) (string, error) {
	resources := cfg.Client().Resources(testNamespace)

	list := &renovatev1beta1.GitRepoList{}
	if err := resources.List(ctx, list); err != nil {
		return "", fmt.Errorf("list GitRepos: %w", err)
	}

	for _, repo := range list.Items {
		if repo.Spec.Name == repoName {
			return repo.Name, nil
		}
	}

	return "", fmt.Errorf("no GitRepo with spec.name %q", repoName)
}

// waitForResourceCondition polls a namespaced resource until the given status
// condition reports True.
func waitForResourceCondition(
	ctx context.Context, cfg *envconf.Config, name string, obj conditionObject, conditionType string,
) error {
	resources := cfg.Client().Resources(testNamespace)

	err := wait.For(func(ctx context.Context) (bool, error) {
		if err := resources.Get(ctx, name, testNamespace, obj); err != nil {
			return false, err
		}

		condition := obj.GetCondition(conditionType)

		return condition != nil && condition.Status == metav1.ConditionTrue, nil
	}, wait.WithContext(ctx), wait.WithTimeout(2*time.Minute), wait.WithInterval(2*time.Second))
	if err != nil {
		return fmt.Errorf("resource %s/%s did not reach condition %q: %w", testNamespace, name, conditionType, err)
	}

	return nil
}
