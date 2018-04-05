package backends

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func TestUpload(t *testing.T) {

	testString := "Test byte upload"

	testBytes := []byte(testString)

	testByteReader := bytes.NewReader(testBytes)

	// var checksumChan = make(chan string, 1)

	hash := sha256.New()

	io.Copy(hash, bytes.NewReader(testBytes))
	// hashString := hex.EncodeToString(hash.Sum(nil))
	hashString := hashBytes(testBytes)

	// hash := generateSHA256Hash(bytes.NewReader(testBytes))
	// checksumChan <- hashString

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		// Ensure the file is equal
		if !bytes.Equal(bodyBytes, testBytes) {
			t.Errorf("Bytes should be equal")
		}

		// Read it back to a string
		if testString != fmt.Sprintf("%s", bodyBytes) {
			t.Errorf("Strings don't match!")
		}

		// Check that the shas match
		checksum := r.Header.Get("X-Amz-Meta-Checksum")
		if hashString != checksum {
			t.Errorf("Checksums should match")
		}

		w.WriteHeader(http.StatusOK)
	})

	session := createTestSetup(handler)

	uploader := &S3Uploader{
		session: session,
		client:  s3.New(session),
		config: &S3Options{
			Versioning:        true,
			ReducedRedundancy: true,
		},
	}

	uploader.UploadFile("test-file", testByteReader, "test-remote", hashString)
}

func TestStartupSync(t *testing.T) {
	// Some initial data
	bytesInSync := []byte("In sync")
	hashInSync := hashBytes(bytesInSync)
	bytesOutOfSync := []byte("Out of sync")
	hashOutOfSync := hashBytes(bytesOutOfSync)

	responseMap := make(map[string]string)

	responseMap["/root/inSync"] = hashInSync
	responseMap["/root/outSync"] = "nothing-hash"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fileHash := responseMap[r.RequestURI]

		if fileHash == "" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			// Set the checksum metadata
			meta := make(map[string]string)

			meta[checksumKey] = fileHash

			w.Header().Set("x-amz-meta-Checksum", fileHash)

			w.WriteHeader(http.StatusOK)
			// json.NewEncoder(w).Encode(resp)
		}
	})

	session := createTestSetup(handler)

	uploader := &S3Uploader{
		session: session,
		client:  s3.New(session),
		config: &S3Options{
			Versioning:        true,
			ReducedRedundancy: true,
		},
	}

	// Try to create a reader and verify the file is in sync
	syncedReader := bytes.NewReader(bytesInSync)
	sync, err := uploader.FileInSync("inSync", "root", syncedReader, hashInSync)
	assert.Nil(t, err, "Error should be nil")
	assert.True(t, sync, "File should be in sync")

	// Out of sync
	outSyncReader := bytes.NewReader(bytesOutOfSync)
	sync, err = uploader.FileInSync("outSync", "root", outSyncReader, hashOutOfSync)
	assert.Nil(t, err, "Should not have error")
	assert.False(t, sync, "Should be out of sync")

	// Missing
	missingReader := bytes.NewReader([]byte("Doesn't exist"))
	sync, err = uploader.FileInSync("missing", "root", missingReader, "no hash")
	assert.Nil(t, err, "Should not have error")
	assert.False(t, sync, "Should be out of sync")

}

func createTestSetup(handler http.HandlerFunc) *session.Session {
	server := httptest.NewServer(handler)

	return session.Must(session.NewSession(aws.NewConfig().
		WithCredentials(credentials.NewStaticCredentials("AKID", "SECRET", "SESSION")).
		WithEndpoint(server.URL).
		WithRegion("mock-region")))
}

func hashBytes(bb []byte) string {
	hash := sha256.New()
	return hex.EncodeToString(hash.Sum(bb))
}
