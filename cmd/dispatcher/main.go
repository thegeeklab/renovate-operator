package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/thegeeklab/renovate-operator/pkg/dispatcher"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	ErrReadFile  = errors.New("failed to read file")
	ErrWriteFile = errors.New("failed to write file")
)

func main() {
	logf.SetLogger(zap.New(zap.JSONEncoder()))

	if err := run(context.Background()); err != nil {
		logf.Log.Error(err, "Failed to run dispatcher")
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	log := logf.FromContext(ctx)

	d, err := dispatcher.New()
	if err != nil {
		return err
	}

	log.Info("Dispatch batch")

	rawConfig, err := os.ReadFile(d.RawConfigFile)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrReadFile, d.RawConfigFile, err)
	}

	log.V(1).Info("Read raw renovate config", "content", rawConfig)

	batchesConfig, err := os.ReadFile(d.BatchesFile)
	if err != nil {
		return fmt.Errorf("%w: %s, %w", ErrReadFile, d.BatchesFile, err)
	}

	log.V(1).Info("Read batches config", "content", batchesConfig)

	mergedConfig, err := d.MergeConfig(rawConfig, batchesConfig, int(d.JobCompletionIndex))
	if err != nil {
		return err
	}

	err = os.WriteFile(d.ConfigFile, mergedConfig, 0o644) //nolint:gosec
	if err != nil {
		return fmt.Errorf("%w: %s, %w", ErrWriteFile, d.ConfigFile, err)
	}

	return nil
}
