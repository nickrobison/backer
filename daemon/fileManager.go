package daemon

import (
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const chmodMask fsnotify.Op = ^fsnotify.Op(0) ^ fsnotify.Chmod

type Uploader interface {
	UploadFile(name string, remotePath string)
	DeleteFile(name string, remotePath string)
}

type Event int

const (
	CREATE Event = 1 + iota
	REMOVE
	WRITE
)

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

type fileManager struct {
	config       *backerConfig
	backlog      Backlog
	uploaders    *[]Uploader
	watcherRoots map[string]string
}

func NewFileManager(config *backerConfig) *fileManager {
	return &fileManager{
		config:       config,
		backlog:      NewMultiFileBacklog(),
		uploaders:    &config.Backends,
		watcherRoots: make(map[string]string),
	}
}

func (f *fileManager) Start(eventChannel <-chan fsnotify.Event, errorChannel <-chan error) {
	fileNameChannel := make(chan BackerEvent)
	batchedChannel := make(chan BackerEvent)
	go f.handleFileEvents(f.config, eventChannel, errorChannel, fileNameChannel)
	go f.batch(fileNameChannel, batchedChannel)
	go f.handleFile(batchedChannel)

}

func (f *fileManager) RegisterWatcherPath(path string, remoteRoot string) {
	if _, ok := f.watcherRoots[path]; ok {
		logger.Printf("Path %s already registered with watcher\n", path)
		return
	}
	f.watcherRoots[path] = remoteRoot
}

func (f *fileManager) handleFileEvents(config *backerConfig, eventChannel <-chan fsnotify.Event, errorChannel <-chan error, outputChannel chan<- BackerEvent) {
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

func (f *fileManager) batch(in <-chan BackerEvent, out chan<- BackerEvent) {
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

func (f *fileManager) handleFile(in <-chan BackerEvent) {
	for event := range in {
		if event.Type == REMOVE {
			logger.Printf("Removing %s from %s\n", event.Path, f.watcherRoots[event.Path])
			uploaderRef := *f.uploaders
			go uploaderRef[0].DeleteFile(event.Path, f.watcherRoots[event.Path])
		} else {
			// Create a wait group
			var wg sync.WaitGroup
			wg.Add(len(*f.uploaders))

			logger.Printf("Uploading %s from %s\n", event.Path, f.watcherRoots[event.Path])
			for _, uploader := range *f.uploaders {
				go func(u Uploader, event *BackerEvent) {
					defer wg.Done()
					u.UploadFile(event.Path, f.watcherRoots[event.Path])
				}(uploader, &event)
			}
			wg.Wait()
			// go f.uploaders.UploadFile(event.Path, f.watcherRoots[event.Path])
		}
	}
}
