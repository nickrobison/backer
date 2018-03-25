package daemon

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"log/syslog"
	"os"
	"os/signal"

	"syscall"

	"github.com/fsnotify/fsnotify"
)

var logger *log.Logger

func init() {
	syslogger, err := syslog.New(syslog.LOG_NOTICE, "backer-daemon")
	if err != nil {
		panic(err)
	}
	logger = log.New(io.MultiWriter(syslogger, os.Stdout), "backer-daemon:", log.Lshortfile)
}

// Start backer daemon
// Registers an S3 uploader and a fileManager to
func Start(configLocation string) {
	logger.Println("Starting up Backer daemon")

	// Read in the config file
	file, err := ioutil.ReadFile(configLocation)
	if err != nil {
		logger.Fatalln("Cannot read config file:", configLocation)
	}

	var config backerConfig
	err = json.Unmarshal(file, &config)
	if err != nil {
		logger.Fatalln(err)
	}

	// Validate the paths
	err = config.validateWatcherPaths()
	if err != nil {
		logger.Fatalln(err)
	}

	// Register the shutdown handler
	done := make(chan bool)
	go shutdown(done)

	// Create a new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Fatalln(err)
	}
	defer watcher.Close()

	// Create new uploader
	s3Client := NewS3Uploader(&config.S3)

	// Register new file manager
	fm := NewFileManager(&config, s3Client)

	// Register all watchers
	for _, newWatcher := range config.Watchers {
		fm.RegisterWatcherPath(newWatcher.GetPath(), newWatcher.BucketPath)
		err = watcher.Add(newWatcher.GetPath())
		if err != nil {
			logger.Fatalln(err)
		}
	}
	fm.Start(watcher.Events, watcher.Errors)

	// Start listener
	go StartSocket(&config)
	logger.Println("Ready to listen")
	<-done
}

func shutdown(done chan bool) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	logger.Println("Shutting down")
	removeSocket()
	// Cleanup code
	done <- true
}