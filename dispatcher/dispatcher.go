package dispatcher

import (
	"encoding/json"
	"fmt"
	"maps"
	"strconv"

	"github.com/thegeeklab/renovate-operator/pkg/util"
)

var (
	ErrInvalidBatchIndex = fmt.Errorf("batch index out of bounds")
	ErrMergeConfig       = fmt.Errorf("failed to merge config")
	ErrDispatcherClient  = fmt.Errorf("failed to create dispatcher client")
)

type Dispatcher struct {
	RawConfigFile      string
	ConfigFile         string
	BatchesFile        string
	JobCompletionIndex int32
	batch              []byte
}

const (
	EnvRenovateRawConfig  = "RENOVATE_RAW_CONFIG"
	EnvRenovateConfig     = "RENOVATE_CONFIG"
	EnvRenovateBatches    = "RENOVATE_BATCHES"
	EnvJobCompletionIndex = "JOB_COMPLETION_INDEX"
)

func New() (*Dispatcher, error) {
	d := &Dispatcher{}

	var err error
	if d.RawConfigFile, err = util.ParseEnv(EnvRenovateRawConfig); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDispatcherClient, err)
	}

	if d.ConfigFile, err = util.ParseEnv(EnvRenovateConfig); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDispatcherClient, err)
	}

	if d.BatchesFile, err = util.ParseEnv(EnvRenovateBatches); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDispatcherClient, err)
	}

	index, err := util.ParseEnv(EnvJobCompletionIndex)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDispatcherClient, err)
	}

	indexInt, err := strconv.ParseInt(index, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse job completion index: %w", err)
	}

	d.JobCompletionIndex = int32(indexInt)

	return d, nil
}

func (d *Dispatcher) MergeConfig(baseConfig, batchConfig []byte, index int) ([]byte, error) {
	var (
		base    map[string]any
		batches []map[string]any
		err     error
	)
	if err := json.Unmarshal(baseConfig, &base); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMergeConfig, err)
	}

	if err := json.Unmarshal(batchConfig, &batches); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMergeConfig, err)
	}

	if index >= len(batches) {
		return nil, fmt.Errorf("%w: %d", ErrInvalidBatchIndex, index)
	}

	d.batch, err = json.Marshal(batches[index])
	if err != nil {
		return nil, err
	}

	merged := maps.Clone(base)
	maps.Copy(merged, batches[index])

	return json.Marshal(merged)
}
