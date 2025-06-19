package main

import (
	"context"
	"fmt"
	"os"

	"github.com/thegeeklab/renovate-operator/jobscheduler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	logf.SetLogger(zap.New(zap.JSONEncoder()))

	if err := run(context.Background()); err != nil {
		logf.Log.Error(err, "Failed to run job scheduler")
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctxLogger := logf.FromContext(ctx)

	js, err := jobscheduler.New()
	if err != nil {
		return fmt.Errorf("failed to create job scheduler: %w", err)
	}

	ctxLogger.Info("Starting job scheduler",
		"renovator", js.RenovatorName,
		"namespace", js.RenovatorNamespace,
		"maxParallelJobs", js.MaxParallelJobs)

	if err := js.CreateRenovatorJobs(ctx); err != nil {
		return fmt.Errorf("failed to create renovator jobs: %w", err)
	}

	ctxLogger.Info("Job scheduler completed successfully")
	return nil
}
