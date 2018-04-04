package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nickrobison/backer/backends"
	"github.com/nickrobison/backer/shared"
)

const chmodMask fsnotify.Op = ^fsnotify.Op(0) ^ fsnotify.Chmod

// Event - Event type from FSNotify
type Event int

const (
	// CREATE - File is created
	CREATE Event = 1 + iota
	// REMOVE - File is removed
	REMOVE
	// WRITE - File is modified
	WRITE
)

// BackerEvent - Event structure which contains a filepath and an event type
type BackerEvent struct {
	Type Event
	Path string
}

func (b *BackerEvent) equals(other BackerEvent) bool {
	if b.Path == other.Path {
		return true
	}

	return false
}

// FileManager - Manages the interaction between FSNotify events and the various data backends
type FileManager struct {
	config       *shared.BackerConfig
	backlog      Backlog
	uploaders    *[]backends.Uploader
	watcherRoots map[string]string
}

// NewFileManager - Helper function for creating a new FileManager
func NewFileManager(config *shared.BackerConfig) *FileManager {
	return &FileManager{
		config:       config,
		backlog:      NewMultiFileBacklog(),
		uploaders:    &config.Backends,
		watcherRoots: make(map[string]string),
	}
}

func (f *FileManager) syncFiles(root string, remotePath string) {
	// If root is a directory, list all the files and check each one individually
	var files []string
	dir, err := isDir(root)
	if err != nil {
		logger.Fatalln(err)
	}

	// If we're a dir, get all the files in the directory
	if dir {
		fls, err := ioutil.ReadDir(root)
		if err != nil {
			logger.Fatalln(err)
		}
		files = make([]string, len(fls))
		for idx, file := range fls {
			files[idx] = filepath.Join(root, file.Name())
		}
	} else {
		files = []string{root}
	}

	var filesWg sync.WaitGroup
	filesWg.Add(len(files))

	// For each file, check that the backends all have the latest copy, or send the new one along
	for _, file := range files {
		func(file string, filesWg *sync.WaitGroup) {
			defer func() {
				logger.Println("Doning filesWG")
				filesWg.Done()
			}()
			var wg sync.WaitGroup
			wg.Add(len(*f.uploaders))

			// Create the writer and checksum array
			writers := make([]io.Writer, len(*f.uploaders))

			// Do the checksum
			checksum, err := f.checksumFile(file)
			if err != nil {
				logger.Fatalln(err)
			}

			for idx, backend := range *f.uploaders {
				br, bw := io.Pipe()
				writers[idx] = bw
				go func(file string, remotePath string, backend backends.Uploader) {
					defer func() {
						logger.Println("Closing backend")
						wg.Done()
					}()
					// checksum := <-checksumChan
					oldChecksum, err := backend.FileInSync(file, remotePath, br, checksum)
					if err != nil {
						logger.Fatalln(err)
					}
					if oldChecksum != "hello" {
						logger.Printf("Updated file %s on backend %s\n", file, backend.GetName())
					}
				}(file, remotePath, backend)
			}

			// hashr, hashw := io.Pipe()

			// hash := sha256.New()
			// writers[len(writers)-1] = hashw

			// go func() {

			// 	hash := sha256.New()
			// 	written, err := io.Copy(hash, hashr)
			// 	if err != nil {
			// 		logger.Println(err)
			// 	}
			// 	logger.Printf("Wrote %d bytes\n", written)

			// 	// ioutil.ReadAll()

			// 	// br, err := ioutil.ReadAll(hash)
			// 	// if err != nil {
			// 	// 	logger.Fatalln(err)
			// 	// }

			// 	// logger.Printf("Hash routine has %d bytes\n", len(br))
			// 	// br2, err := hash.Write(br)
			// 	// if err != nil {
			// 	// 	logger.Fatalln(err)
			// 	// }
			// 	// logger.Printf("Hashed %d bytes\n", hash.Size())
			// 	encoded := hex.EncodeToString(hash.Sum(nil))
			// 	logger.Println()
			// 	for _, channel := range chanArray {
			// 		logger.Println("Sending hash")
			// 		channel <- encoded
			// 	}
			// }()

			go f.processFile(&writers, file)

			wg.Wait()
			logger.Printf("Finished syncing %s to backends\n", file)
		}(file, &filesWg)
	}
	filesWg.Wait()
	logger.Println("Sync has finished")
}

// Start - Start watching for File events
func (f *FileManager) Start(eventChannel <-chan fsnotify.Event, errorChannel <-chan error) {
	// Before starting everything, check to ensure that our initial state is up to date, if that's what we're configured to do
	if f.config.SyncOnStartup {
		logger.Println("Synchronizing file roots with backend")
		for path, remote := range f.watcherRoots {
			f.syncFiles(path, remote)
		}
	}

	fileNameChannel := make(chan BackerEvent)
	batchedChannel := make(chan BackerEvent)
	go f.handleFileEvents(f.config, eventChannel, errorChannel, fileNameChannel)
	go f.batch(fileNameChannel, batchedChannel)
	go f.handleFile(batchedChannel)

}

// RegisterWatcherPath - Register a file path with the Manager, will subscribe to FSEvents for this path
func (f *FileManager) RegisterWatcherPath(path string, remoteRoot string) {
	if _, ok := f.watcherRoots[path]; ok {
		logger.Printf("Path %s already registered with watcher\n", path)
		return
	}
	f.watcherRoots[path] = remoteRoot
}

func (f *FileManager) handleFileEvents(config *shared.BackerConfig, eventChannel <-chan fsnotify.Event, errorChannel <-chan error, outputChannel chan<- BackerEvent) {
	logger.Println("Launching new file handler")
	for {
		select {
		case event := <-eventChannel:
			{
				if event.Op&chmodMask == 0 {
					continue
				}
				logger.Printf("Has event: %v", event)
				if event.Op == fsnotify.Remove {
					if f.config.DeleteOnRemove {
						outputChannel <- BackerEvent{
							Type: REMOVE,
							Path: event.Name,
						}
						continue
					}
					logger.Printf("Removed file %s, continuing\n", event.Name)
					continue
				}
				outputChannel <- BackerEvent{
					Type: CREATE,
					Path: event.Name,
				}
			}
		case err := <-errorChannel:
			{
				// When the application shutsdown
				if err != nil {
					logger.Fatalln(err)
				}
			}
		}
	}
}

func (f *FileManager) batch(in <-chan BackerEvent, out chan<- BackerEvent) {
	logger.Println("Starting batch process")
	for event := range in {
		f.backlog.Add(event)
		timer := time.NewTimer(300 * time.Millisecond)
	outer:
		for {
			select {
			case event := <-in:
				f.backlog.Add(event)
			case <-timer.C:
				for {
					select {
					case event := <-in:
						f.backlog.Add(event)
					case out <- f.backlog.Next():
						if f.backlog.RemoveOne() {
							break outer
						}
					}
				}
			}
		}
	}
}

func (f *FileManager) handleFile(in <-chan BackerEvent) {
	for event := range in {
		if event.Type == REMOVE {
			logger.Printf("Removing %s from %s\n", event.Path, f.watcherRoots[event.Path])
			uploaderRef := *f.uploaders
			go uploaderRef[0].DeleteFile(event.Path, f.watcherRoots[event.Path])
		} else {
			f.handleFileUpload(&event)
		}
	}
}

func (f *FileManager) handleFileUpload(event *BackerEvent) {
	// Create a wait group to synchronize all the backends and the checksum goroutine
	var wg sync.WaitGroup
	wg.Add(len(*f.uploaders))

	uploaderRef := f.uploaders
	var pipeWriters = make([]io.Writer, len(*uploaderRef))

	watcherPath := f.watcherRoots[event.Path]

	// Do the checksumming
	checksum, err := f.checksumFile(event.Path)
	if err != nil {
		logger.Fatalln(err)
	}

	logger.Printf("Uploading %s to %s\n", event.Path, watcherPath)
	for idx, uploader := range *f.uploaders {
		// For each uploader, create a new pipe writer
		reader, writer := io.Pipe()
		pipeWriters[idx] = writer

		go func(u backends.Uploader, event *BackerEvent) {
			defer wg.Done()
			u.UploadFile(event.Path, reader, watcherPath, checksum)
		}(uploader, event)
	}

	// Create a new Hash function to checksum the file
	// hashr, hashw := io.Pipe()
	// hash := sha256.New()
	// pipeWriters[len(pipeWriters)-1] = hashw

	// Run this in a go routine, so that way when it returns, we close all the writers, otherwise they'll deadlock and never stop reading
	go f.processFile(&pipeWriters, event.Path)
	// go func() {

	// }()
	wg.Wait()
	// Unlock the file
	// logger.Println("Unlocking file")
	logger.Printf("Finished uploading %s\n", event.Path)
}

func (f *FileManager) checksumFile(filename string) (string, error) {

	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}

	// Get the hash value and send it along to the backends
	hashString := hex.EncodeToString(hash.Sum(nil))
	logger.Println(hashString)
	return hashString, nil
}

func (f *FileManager) processFile(pipeWriters *[]io.Writer, filename string) {
	// Read in the file
	// Should I lock this file?
	file, err := os.Open(filename)
	if err != nil {
		logger.Println(err)
	}
	defer file.Close()

	// Defer closing all the writers, except the Hash
	for _, writer := range *pipeWriters {
		// Cast this back to a PipeWriter, seems gross
		w, ok := writer.(*io.PipeWriter)
		if ok {
			defer w.Close()
		}
	}

	// Create a multiwriter and read everything into it
	mw := io.MultiWriter(*pipeWriters...)
	logger.Println("Reading from file")
	bytes, err := io.Copy(mw, file)
	if err != nil {
		logger.Fatalln(err)
	}
	logger.Printf("Finished reading %d bytes to pipes\n", bytes)
}

func isDir(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	if fi.Mode().IsDir() {
		return true, nil
	}
	return false, nil
}
