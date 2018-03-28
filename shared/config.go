package shared

import (
	"os"
	"path/filepath"

	"github.com/nickrobison/backer/backends"
)

type pathError struct {
	location string
}

func (e *pathError) Error() string {
	return "Resource: " + e.location + " does not exist"
}

// Watcher - Configuration struct for watching a specific file path
type Watcher struct {
	BucketPath string `json:"bucketPath"`
	Path       string `json:"path"`
}

// GetPath - Returns the absolute Path of the Watcher
func (w *Watcher) GetPath() (string, error) {
	path, err := filepath.Abs(w.Path)
	if err != nil {
		return "", err
	}

	return path, nil
}

// BackerConfig - Main configuration struct
type BackerConfig struct {
	SyncOnStartup    bool               `json:"syncOnStartup"`
	DeleteOnRemove   bool               `json:"deleteOnRemove"`
	DeleteOnShutdown bool               `json:"deleteOnShutdown"`
	Watchers         []Watcher          `json:"watchers"`
	S3               backends.S3Options `json:"s3"`
	Backends         []backends.Uploader
}

// ValidateWatcherPaths - Ensure that each path in the config file is valid and exists
func (c *BackerConfig) ValidateWatcherPaths() error {
	for _, pathWatcher := range c.Watchers {
		path, err := pathWatcher.GetPath()
		if err != nil {
			return err
		}
		// Ensure it exists
		_, err = os.Stat(path)
		if err != nil {
			return &pathError{
				location: path,
			}
		}
	}
	return nil
}
