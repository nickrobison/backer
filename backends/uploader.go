package backends

import "io"

// Uploader - Primary interface to be implemented by the various backends
type Uploader interface {
	UploadFile(name string, data io.Reader, remotePath string, checksum string)
	FileInSync(name string, remotePath string, data io.Reader, checksum string) (string, error)
	DeleteFile(name string, remotePath string)
	GetName() string
}
