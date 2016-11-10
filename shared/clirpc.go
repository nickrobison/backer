package shared

type Args struct {
	objectKey string
}

type FileWatchers struct {
	Paths []string
}

type BucketObjects struct {
	Key     string
	Version []string
}

type CLICommunication interface {
	SayHello(args int, hello *string) error
	ListWatchers(args int, watchers *FileWatchers) error
	ListObjects(objects *[]BucketObjects) error
	ListObjectVersions(args *Args, object *BucketObjects) error
}
