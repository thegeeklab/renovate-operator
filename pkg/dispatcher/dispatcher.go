package dispatcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps" // Added os import
	"strconv"

	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util"
)

var (
	ErrInvalidIndex     = errors.New("index out of bounds")
	ErrMergeConfig      = errors.New("failed to merge config")
	ErrDispatcherClient = errors.New("failed to create dispatcher client")
)

// Define the standard Kubernetes termination log path.
const TerminationLogFile = "/dev/termination-log"

type Dispatcher struct {
	RawConfigFile      string
	ConfigFile         string
	IndexFile          string
	JobCompletionIndex int32
}

func New() (*Dispatcher, error) {
	d := &Dispatcher{}

	var err error
	if d.RawConfigFile, err = util.ParseEnv(renovate.EnvRenovateConfigRaw); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDispatcherClient, err)
	}

	if d.ConfigFile, err = util.ParseEnv(renovate.EnvRenovateConfig); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDispatcherClient, err)
	}

	if d.IndexFile, err = util.ParseEnv(renovate.EnvRenovateIndex); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDispatcherClient, err)
	}

	index, err := util.ParseEnv(renovate.EnvJobCompletionIndex)
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

func (d *Dispatcher) MergeConfig(baseConfig, jobConfig []byte, index int) ([]byte, error) {
	var (
		base        map[string]any
		indexConfig []map[string]any
	)

	if err := json.Unmarshal(baseConfig, &base); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMergeConfig, err)
	}

	if err := json.Unmarshal(jobConfig, &indexConfig); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMergeConfig, err)
	}

	if index >= len(indexConfig) {
		return nil, fmt.Errorf("%w: %d", ErrInvalidIndex, index)
	}

	merged := maps.Clone(base)
	maps.Copy(merged, indexConfig[index])

	return json.Marshal(merged)
}
