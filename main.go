package main

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
	syslogger, err := syslog.New(syslog.LOG_NOTICE, "backer")
	if err != nil {
		panic(err)
	}
	logger = log.New(io.MultiWriter(syslogger, os.Stdout), "backer", log.Lshortfile)
}

func main() {
	logger.Println("Starting up")

	// Read in the config file
	file, err := ioutil.ReadFile("./config.json")
	if err != nil {
		logger.Fatalln("Missing config file")
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
	<-done
}

func shutdown(done chan bool) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	logger.Println("Shutting down")
	// Cleanup code
	done <- true
}
