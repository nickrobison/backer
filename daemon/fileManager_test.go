package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/nickrobison/backer/backends"
	"github.com/nickrobison/backer/shared"
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
		path, err := newWatcher.GetPath()
		if err != nil {
			logger.Fatalln(err)
		}
		fm.RegisterWatcherPath(path, newWatcher.BucketPath)
		err = watcher.Add(path)
		assert.Nil(t, err, "Should be able to register watchers")
	}
	go fm.Start(watcher.Events, watcher.Errors)
	tmpf := filepath.Join(dir, "tempFile")
	// Try to create a new file
	err = ioutil.WriteFile(tmpf, []byte(""), 0666)
	assert.Nil(t, err, "Should be able to write")

	hashString := hashData(t, []byte(""))
	<-mb.done
	// Verify that things are correct
	assert.Equal(t, 0, mb.dataSize, "Should have zero byte file")
	assert.Equal(t, "", mb.dataContent, "Should have empty file contents")
	assert.Equal(t, hashString, mb.checksum, "Should have matching checksum")

	// Now try to add more data
	err = ioutil.WriteFile(tmpf, []byte("Hi there!"), 0666)
	assert.Nil(t, err, "Should be able to write")

	hashString = hashData(t, []byte("Hi there!"))
	<-mb.done
	assert.Equal(t, 9, mb.dataSize, "Should have 9 byte file")
	assert.Equal(t, "Hi there!", mb.dataContent, "Should have updated contents")
	assert.Equal(t, hashString, mb.checksum, "Should have matching checksum")

	// Try to write a new file with different data
	testString := "There are many things in this world, that I love"
	tmpf2 := filepath.Join(dir, "tempFile2")
	// Try to create a new file
	err = ioutil.WriteFile(tmpf2, []byte(testString), 0666)
	assert.Nil(t, err, "Should be able to write")

	hashString = hashData(t, []byte(testString))
	<-mb.done
	assert.Equal(t, len(testString), mb.dataSize, "Should have new, larger file")
	assert.Equal(t, testString, mb.dataContent, "Should have matching data content")
	assert.Equal(t, hashString, mb.checksum, "Should have matching data")

	// Now, try to delete the latest file
	err = os.Remove(tmpf2)
	assert.Nil(t, err, "Should be able to delete the file")

	<-mb.done
	assert.Equal(t, tmpf2, mb.deletedFile, "Should delete 2nd temp file")
}

func TestDirectorySync(t *testing.T) {
	// Create our mock backend with some initial values
	mb := &MockBackend{
		done:              make(chan bool),
		dataContent:       "All wrong",
		dataSize:          1024,
		checksum:          "Wrong checksum",
		synchronizedFiles: make(map[string]string),
	}

	// Create a new file manager
	fm, dir := createFileManager(mb)
	defer os.RemoveAll(dir)
	// Set sync to be true
	fm.config.SyncOnStartup = true

	// Create a new watcher
	watcher, err := fsnotify.NewWatcher()
	assert.Nil(t, err, "Should be able to create watcher")
	defer watcher.Close()

	// Register the watchers
	for _, newWatcher := range fm.config.Watchers {
		path, err := newWatcher.GetPath()
		if err != nil {
			logger.Fatalln(err)
		}
		fm.RegisterWatcherPath(path, newWatcher.BucketPath)
		err = watcher.Add(path)
		assert.Nil(t, err, "Should be able to register watchers")
	}

	// Create some files and then start the watcher
	tmpf := filepath.Join(dir, "tempFile")
	err = ioutil.WriteFile(tmpf, []byte("First temp file."), 0666)
	assert.Nil(t, err, "Should be able to write")

	tmpf2 := filepath.Join(dir, "tempFile2")
	err = ioutil.WriteFile(tmpf2, []byte("Second temp file, with more datas."), 0666)
	assert.Nil(t, err, "Should be able to write")

	go fm.Start(watcher.Events, watcher.Errors)

	// Wait for both of the files to be
	for i := 0; i < 2; i++ {
		<-mb.done
	}
	// Check that we have 2 files
	assert.Equal(t, 2, len(mb.synchronizedFiles), "Should have synced 2 files")
	// For each of the above files, compute the hash and make sure it matches
	hash := sha256.New()
	file, err := os.Open(tmpf)
	assert.Nil(t, err, "Should be able to open file")
	_, err = io.Copy(hash, file)
	assert.Nil(t, err, "Should be able to copy hash")
	syncHash := mb.synchronizedFiles[tmpf]
	assert.Equal(t, hex.EncodeToString(hash.Sum(nil)), syncHash, "Hash should match for first file")

	hash = sha256.New()
	file, err = os.Open(tmpf2)
	assert.Nil(t, err, "Should be able to open file")
	_, err = io.Copy(hash, file)
	assert.Nil(t, err, "Should be able to copy hash")
	syncHash = mb.synchronizedFiles[tmpf2]
	assert.Equal(t, hex.EncodeToString(hash.Sum(nil)), syncHash, "Hash should match for second file")
}

func createFileManager(backend backends.Uploader) (*FileManager, string) {

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
	var watchers = []shared.Watcher{shared.Watcher{
		BucketPath: "test-bucket",
		Path:       dir,
	}}

	var backends = []backends.Uploader{backend}

	var mockConfig = &shared.BackerConfig{
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
	done              chan bool
	checksum          string
	dataSize          int
	dataContent       string
	deletedFile       string
	synchronizedFiles map[string]string
}

func (b *MockBackend) UploadFile(name string, data io.Reader, remotePath string, checksum string) {
	bytes, err := ioutil.ReadAll(data)
	if err != nil {
		panic(err)
	}
	byteData := string(bytes)
	byteLen := len(bytes)
	b.dataSize = byteLen
	b.dataContent = byteData
	b.checksum = checksum
	b.done <- true
}

func (b *MockBackend) DeleteFile(name string, remotePath string) {
	b.deletedFile = name
	b.done <- true
}

func (b *MockBackend) GetName() string {
	return "MockBackend"
}

func (b *MockBackend) FileInSync(name string, remotePath string, data io.Reader, checksum string) (bool, error) {
	bytes, err := ioutil.ReadAll(data)
	if err != nil {
		panic(err)
	}
	log.Printf("Mock backend: %s\n", string(bytes))
	b.synchronizedFiles[name] = checksum
	return true, nil
}

func hashData(t *testing.T, data []byte) string {
	hash := sha256.New()
	_, err := hash.Write(data)
	assert.Nil(t, err, "Should be able to hash")
	return hex.EncodeToString(hash.Sum(nil))
}
