package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"os"
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

// Start - Start watching for File events
func (f *FileManager) Start(eventChannel <-chan fsnotify.Event, errorChannel <-chan error) {
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
	var pipeWriters = make([]io.Writer, len(*uploaderRef)+1)
	var checksumChannels = make([]chan string, len(*uploaderRef))

	watcherPath := f.watcherRoots[event.Path]

	logger.Printf("Uploading %s to %s\n", event.Path, watcherPath)
	for idx, uploader := range *f.uploaders {
		// For each uploader, create a new pipe writer
		reader, writer := io.Pipe()
		pipeWriters[idx] = writer

		// Make a checksum channel
		var checksumChan = make(chan string, 2)
		checksumChannels[idx] = checksumChan

		go func(u backends.Uploader, event *BackerEvent) {
			defer wg.Done()
			u.UploadFile(event.Path, reader, watcherPath, checksumChan)
		}(uploader, event)
	}

	// Create a new Hash function to checksum the file
	hash := sha256.New()
	pipeWriters[len(pipeWriters)-1] = hash

	// Do the checksumming
	go f.checksumFile(hash, &wg, &checksumChannels)

	// Run this in a go routine, so that way when it returns, we close all the writers, otherwise they'll deadlock and never stop reading
	go f.processFile(&pipeWriters, event.Path)
	// go func() {

	// }()
	wg.Wait()
	logger.Printf("Wait done, closing %d channels", len(checksumChannels))
	for _, channel := range checksumChannels {
		close(channel)
	}
	// Unlock the file
	// logger.Println("Unlocking file")
	logger.Printf("Finished uploading %s\n", event.Path)
}

func (f *FileManager) checksumFile(hash hash.Hash, wg *sync.WaitGroup, checksumChannels *[]chan string) {

	wg.Add(1)
	defer wg.Done()

	// Get the hash value and send it along to the backends
	hashString := hex.EncodeToString(hash.Sum(nil))
	logger.Println(hashString)
	for _, channel := range *checksumChannels {
		channel <- hashString
	}
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
	io.Copy(mw, file)
	logger.Println("Finished reading to pipes")
}
