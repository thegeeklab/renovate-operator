package logstore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const logExt = ".log"

// FileStore implements the Store interface using a local filesystem or mounted PVC.
type FileStore struct {
	baseDir string
}

// NewFileStore initializes a new FileStore back-end.
// Ensure the baseDir exists and is writable by the operator process.
func NewFileStore(baseDir string) *FileStore {
	return &FileStore{
		baseDir: baseDir,
	}
}

// SaveLog streams the K8s pod logs directly into a file on the PVC.
func (f *FileStore) SaveLog(
	ctx context.Context, namespace, component, owner, jobName string, logReader io.Reader,
) error {
	dir := filepath.Join(f.baseDir, namespace, component, owner)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	filePath := filepath.Join(dir, jobName+logExt)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, logReader); err != nil {
		return fmt.Errorf("failed to write logs to file: %w", err)
	}

	return nil
}

// GetLog opens the log file and returns it. The caller must close it.
func (f *FileStore) GetLog(ctx context.Context, namespace, component, owner, jobName string) (io.ReadCloser, error) {
	filePath := filepath.Join(f.baseDir, namespace, component, owner, jobName+logExt)

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrLogNotFound
		}

		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return file, nil
}

// ListLogs scans the owner's directory and returns metadata for all stored logs.
func (f *FileStore) ListLogs(ctx context.Context, namespace, component, owner string) ([]LogEntry, error) {
	dir := filepath.Join(f.baseDir, namespace, component, owner)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogEntry{}, nil
		}

		return nil, fmt.Errorf("failed to read log directory: %w", err)
	}

	var logs []LogEntry

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), logExt) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		jobName := strings.TrimSuffix(entry.Name(), logExt)

		logs = append(logs, LogEntry{
			Namespace: namespace,
			Component: component,
			Owner:     owner,
			JobName:   jobName,
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].CreatedAt.After(logs[j].CreatedAt)
	})

	return logs, nil
}

// DeleteLog removes a specific log file.
func (f *FileStore) DeleteLog(ctx context.Context, namespace, component, owner, jobName string) error {
	filePath := filepath.Join(f.baseDir, namespace, component, owner, jobName+logExt)

	err := os.Remove(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("failed to delete log file: %w", err)
	}

	return nil
}
