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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func TestUpload(t *testing.T) {

	testString := "Test byte upload"

	testBytes := []byte(testString)

	testByteReader := bytes.NewReader(testBytes)

	var checksumChan = make(chan string, 1)

	hash := sha256.New()

	io.Copy(hash, bytes.NewReader(testBytes))
	hashString := hex.EncodeToString(hash.Sum(nil))

	// hash := generateSHA256Hash(bytes.NewReader(testBytes))
	checksumChan <- hashString

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

	uploader.UploadFile("test-file", testByteReader, "test-remote", checksumChan)
}

func createTestSetup(handler http.HandlerFunc) *session.Session {
	server := httptest.NewServer(handler)

	return session.Must(session.NewSession(aws.NewConfig().
		WithCredentials(credentials.NewStaticCredentials("AKID", "SECRET", "SESSION")).
		WithEndpoint(server.URL).
		WithRegion("mock-region")))
}
