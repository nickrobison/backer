package shared

// Args - simple args struct
type Args struct {
	objectKey string
}

// FileWatchers - List of current file paths
type FileWatchers struct {
	Paths []string
}

// BucketObjects - An object and its given versions
type BucketObjects struct {
	Key     string
	Version []string
}

// CLICommunication - basic interface for communicating between the cli and the backend
type CLICommunication interface {
	ListWatchers(args int, watchers *FileWatchers) error
	ListObjects(objects *[]BucketObjects) error
	ListObjectVersions(args *Args, object *BucketObjects) error
}
