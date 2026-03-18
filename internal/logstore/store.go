package logstore

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrLogNotFound is returned when a requested log file does not exist in the store.
var ErrLogNotFound = errors.New("log not found")

// LogEntry contains metadata about a stored log.
type LogEntry struct {
	Namespace string    `json:"namespace"`
	Component string    `json:"component"` // e.g., "runner" or "discovery"
	Owner     string    `json:"owner"`     // e.g., the GitRepo name or Discovery name
	JobName   string    `json:"jobName"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt"`
}

// Store defines the interface for persistently managing runner logs.
// This decouples log storage from the Kubernetes resource lifecycle, allowing
// logs to be retained even after Jobs and Pods are pruned.
type Store interface {
	// SaveLog reads from the provided logReader (e.g., a Pod log stream)
	// and stores the log persistently.
	SaveLog(ctx context.Context, namespace, component, owner, jobName string, logReader io.Reader) error

	// GetLog returns an io.ReadCloser to stream a specific run's archived log.
	// Returns ErrLogNotFound if the log does not exist.
	// The caller is strictly responsible for closing the reader.
	GetLog(ctx context.Context, namespace, component, owner, jobName string) (io.ReadCloser, error)

	// ListLogs returns all available logs for a specific owner/component,
	// sorted from newest to oldest.
	ListLogs(ctx context.Context, namespace, component, owner string) ([]LogEntry, error)

	// DeleteLog removes a specific log from the store (useful for custom retention/cleanup policies).
	DeleteLog(ctx context.Context, namespace, component, owner, jobName string) error
}
