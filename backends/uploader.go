package backends

import "io"

// Uploader - Primary interface to be implemented by the various backends
type Uploader interface {
	UploadFile(name string, data io.Reader, remotePath string, checksumChannel chan string)
	DeleteFile(name string, remotePath string)
}
