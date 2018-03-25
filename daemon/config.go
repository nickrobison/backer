package daemon

import (
	"os"
	"path/filepath"
)

type pathError struct {
	location string
}

func (e *pathError) Error() string {
	return "Resource: " + e.location + " does not exist"
}

type Watcher struct {
	BucketPath string `json:"bucketPath"`
	Path       string `json:"path"`
}

func (w *Watcher) GetPath() string {
	path, err := filepath.Abs(w.Path)
	if err != nil {
		logger.Fatalln(err)
	}

	return path
}

type backerConfig struct {
	DeleteOnRemove   bool      `json:"deleteOnRemove"`
	DeleteOnShutdown bool      `json:"deleteOnShutdown"`
	Watchers         []Watcher `json:"watchers"`
	S3               s3Options `json:"s3"`
	Backends         []Uploader
}

type s3Options struct {
	Region            string        `json:"region"`
	Bucket            string        `json:"bucket"`
	BucketRoot        string        `json:"bucketRoot"`
	Credentials       s3Credentials `json:"credentials"`
	Versioning        bool          `json:"versioning"`
	ReducedRedundancy bool          `json:"reducedRedundancy"`
}

type s3Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	ProviderName    string
}

func (c *backerConfig) validateWatcherPaths() error {
	for _, pathWatcher := range c.Watchers {
		path, err := filepath.Abs(pathWatcher.Path)
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
