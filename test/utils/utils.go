// Package utils provides shared helpers for the end-to-end test suite.
package utils

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const (
	// OperatorNamespace is the namespace the operator is deployed into by config/default.
	OperatorNamespace = "renovate-system"
	// ManagerDeploymentName is the name of the controller-manager Deployment.
	ManagerDeploymentName = "renovate-operator-controller-manager"
	// WebhookConfigurationName is the name of the mutating/validating webhook configuration.
	WebhookConfigurationName = "renovate-operator-webhook-configuration"

	configDir = "config/default"
)

// RepoRoot returns the absolute path of the repository root, derived from the
// location of this source file so it is independent of the test working directory.
func RepoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// RunCommand executes name with args in dir and returns an error that includes
// the combined output when the command fails.
func RunCommand(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		command := strings.Join(append([]string{name}, args...), " ")

		return fmt.Errorf("%s failed: %w\n%s", command, err, strings.TrimSpace(string(output)))
	}

	return nil
}

// ManagerLogs returns the most recent log lines from the operator manager
// deployment. It is used to enrich e2e failure messages with the reconcile
// errors the operator logged, so failures can be diagnosed without collecting
// logs by hand. It returns a placeholder string when the logs cannot be
// retrieved, making it always safe to embed in a failure message.
func ManagerLogs(ctx context.Context) string {
	args := []string{
		"logs",
		"-n", OperatorNamespace,
		"deployment/" + ManagerDeploymentName,
		"--all-containers=true",
		"--tail=100",
		"--prefix=true",
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("<unable to fetch manager logs: %v>", err)
	}

	return strings.TrimSpace(string(output))
}

// BuildImage builds the operator container image and tags it with image.
func BuildImage(image string) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		if err := RunCommand(ctx, RepoRoot(), "make", "docker-build", "IMG="+image); err != nil {
			return ctx, fmt.Errorf("build operator image: %w", err)
		}

		return ctx, nil
	}
}

// DeployOperator installs the operator into the cluster using config/default
// and points the manager Deployment at the given locally built image.
func DeployOperator(image string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		applyArgs := []string{"apply", "--server-side", "--force-conflicts", "-k", configDir}
		if err := RunCommand(ctx, RepoRoot(), "kubectl", applyArgs...); err != nil {
			return ctx, fmt.Errorf("deploy operator: %w", err)
		}

		if err := setManagerImage(ctx, cfg, image); err != nil {
			return ctx, fmt.Errorf("set manager image: %w", err)
		}

		return ctx, nil
	}
}

// UndeployOperator removes the operator and its namespace from the cluster. It
// is best-effort and intended for use in the environment teardown step.
func UndeployOperator() env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		deleteArgs := []string{"delete", "-k", configDir, "--ignore-not-found=true", "--wait=false"}
		if err := RunCommand(ctx, RepoRoot(), "kubectl", deleteArgs...); err != nil {
			return ctx, fmt.Errorf("undeploy operator: %w", err)
		}

		return ctx, nil
	}
}

func setManagerImage(ctx context.Context, cfg *envconf.Config, image string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		deployment := &appsv1.Deployment{
			Name:      ManagerDeploymentName,
			Namespace: OperatorNamespace,
		}

		resources := cfg.Client().Resources(OperatorNamespace)
		if err := resources.Get(ctx, ManagerDeploymentName, OperatorNamespace, deployment); err != nil {
			return fmt.Errorf("get manager deployment: %w", err)
		}

		deployment.Spec.Template.Spec.Containers[0].Image = image
		deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullIfNotPresent
		// The manager runs single-replica with leader election, so a rolling
		// update deadlocks: the new pod never becomes ready because the
		// frontend server only starts once it acquires leadership, which the
		// old pod holds. Recreate terminates the old pod first.
		deployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType
		deployment.Spec.Strategy.RollingUpdate = nil

		if err := resources.Update(ctx, deployment); err != nil {
			return fmt.Errorf("update manager deployment: %w", err)
		}

		return nil
	})
}
