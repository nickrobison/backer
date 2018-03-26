package daemon

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const chmodMask fsnotify.Op = ^fsnotify.Op(0) ^ fsnotify.Chmod

// Uploader - Primary interface to be implemented by the various backends
type Uploader interface {
	UploadFile(name string, data io.Reader, remotePath string)
	DeleteFile(name string, remotePath string)
}

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
	config       *backerConfig
	backlog      Backlog
	uploaders    *[]Uploader
	watcherRoots map[string]string
}

// NewFileManager - Helper function for creating a new FileManager
func NewFileManager(config *backerConfig) *FileManager {
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

func (f *FileManager) handleFileEvents(config *backerConfig, eventChannel <-chan fsnotify.Event, errorChannel <-chan error, outputChannel chan<- BackerEvent) {
	logger.Println("Launching new file handler")
	for {
		select {
		case event := <-eventChannel:
			{
				if event.Op&chmodMask == 0 {
					continue
				}
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
			// Create a wait group to synchronize all the backends
			var wg sync.WaitGroup
			wg.Add(len(*f.uploaders))

			// Read in the file
			// Should I lock this file?
			file, err := os.Open(event.Path)
			if err != nil {
				logger.Println(err)
			}
			defer file.Close()

			uploaderRef := f.uploaders
			var pipeWriters = make([]io.Writer, len(*uploaderRef))

			watcherPath := f.watcherRoots[event.Path]

			logger.Printf("Uploading %s to %s\n", event.Path, watcherPath)
			for idx, uploader := range *f.uploaders {
				// For each uploader, create a new pipe writer
				reader, writer := io.Pipe()
				pipeWriters[idx] = writer

				go func(u Uploader, event *BackerEvent) {
					defer wg.Done()
					u.UploadFile(event.Path, reader, watcherPath)
				}(uploader, &event)
			}

			// Run this in a go routine, so that way when it returns, we close all the writers, otherwise they'll deadlock and never stop reading
			go func() {
				// Close all the writers
				for _, writer := range pipeWriters {
					// Cast this back to a PipeWriter, seems gross
					defer writer.(*io.PipeWriter).Close()
				}
				// Create a multiwriter and read everything into it
				mw := io.MultiWriter(pipeWriters...)
				io.Copy(mw, file)
			}()
			wg.Wait()
		}
	}
}

func closeWriter(writer *io.PipeWriter) {
	logger.Println("Closing Writer")
	writer.Close()
}

func closeReader(reader *io.PipeReader) {
	logger.Println("Closing Readiner")
	reader.Close()
}
