package daemon

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fsnotify/fsnotify"
)

func TestFileCreation(t *testing.T) {
	// Create our mock backend with some initial values
	mb := &MockBackend{
		done:        make(chan bool),
		dataContent: "All wrong",
		dataSize:    1024,
		checksum:    "Wrong checksum",
	}

	// Create a new file manager
	fm, dir := createFileManager(mb)
	defer os.RemoveAll(dir)

	// Create a new watcher
	watcher, err := fsnotify.NewWatcher()
	assert.Nil(t, err, "Should be able to create watcher")
	defer watcher.Close()

	// Register the watchers
	for _, newWatcher := range fm.config.Watchers {
		fm.RegisterWatcherPath(newWatcher.GetPath(), newWatcher.BucketPath)
		err = watcher.Add(newWatcher.GetPath())
		assert.Nil(t, err, "Should be able to register watchers")
	}

	go fm.Start(watcher.Events, watcher.Errors)

	tmpf := filepath.Join(dir, "tempFile")
	// Try to create a new file
	err = ioutil.WriteFile(tmpf, []byte(""), 0666)
	assert.Nil(t, err, "Should be able to write")

	<-mb.done
	// Verify that things are correct
	assert.Equal(t, 0, mb.dataSize, "Should have zero byte file")
	assert.Equal(t, "", mb.dataContent, "Should have empty file contents")

	// Now try to add more data
	err = ioutil.WriteFile(tmpf, []byte("Hi there!"), 0666)
	assert.Nil(t, err, "Should be able to write")

	<-mb.done
	assert.Equal(t, 9, mb.dataSize, "Should have 9 byte file")
	assert.Equal(t, "Hi there!", mb.dataContent, "Should have updated contents")

	// Try to write a new file with different data
	testString := "There are many things in this world, that I love"
	tmpf2 := filepath.Join(dir, "tempFile2")
	// Try to create a new file
	err = ioutil.WriteFile(tmpf2, []byte(testString), 0666)
	assert.Nil(t, err, "Should be able to write")

	<-mb.done
	assert.Equal(t, len(testString), mb.dataSize, "Should have new, larger file")
	assert.Equal(t, testString, mb.dataContent, "Should have matching data content")

	// Now, try to delete the latest file
	err = os.Remove(tmpf2)
	assert.Nil(t, err, "Should be able to delete the file")

	<-mb.done
	assert.Equal(t, tmpf2, mb.deletedFile, "Should delete 2nd temp file")
}

func createFileManager(backend Uploader) (*FileManager, string) {

	// Create a folder to watch
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	dir, err := ioutil.TempDir(wd, "backer-test")
	if err != nil {
		panic(err)
	}

	// Create the watchers
	var watchers = []Watcher{Watcher{
		BucketPath: "test-bucket",
		Path:       dir,
	}}

	var backends = []Uploader{backend}

	var mockConfig = &backerConfig{
		DeleteOnRemove: true,
		Backends:       backends,
		Watchers:       watchers,
	}

	fm := &FileManager{
		config:       mockConfig,
		uploaders:    &mockConfig.Backends,
		backlog:      NewMultiFileBacklog(),
		watcherRoots: make(map[string]string),
	}

	return fm, dir
}

type MockBackend struct {
	done        chan bool
	checksum    string
	dataSize    int
	dataContent string
	deletedFile string
}

func (b *MockBackend) UploadFile(name string, data io.Reader, remotePath string, checksumChannel chan string) {
	b.checksum = <-checksumChannel
	bytes, err := ioutil.ReadAll(data)
	if err != nil {
		panic(err)
	}
	byteData := string(bytes)
	byteLen := len(bytes)
	b.dataSize = byteLen
	b.dataContent = byteData
	b.done <- true
}

func (b *MockBackend) DeleteFile(name string, remotePath string) {
	b.deletedFile = name
	b.done <- true
}
