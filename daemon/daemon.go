package daemon

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"
	"time"

	systemd "github.com/coreos/go-systemd/daemon"
	"github.com/fsnotify/fsnotify"
)

var logger *log.Logger

func init() {
	logger = log.New(os.Stdout, "backer-daemon:", log.Lshortfile)
}

// Start backer daemon
// Registers an S3 uploader and a fileManager to watch the watchers
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
	var clients []Uploader

	s3Client := NewS3Uploader(&config.S3)
	clients = append(clients, s3Client)
	config.Backends = clients

	// Register new file manager
	fm := NewFileManager(&config)

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
	l, err := getSocket()
	if err != nil {
		logger.Fatalln(err)
	}
	defer l.Close()

	cliRPC := &RPC{
		Config: &config,
	}
	server := rpc.NewServer()
	server.RegisterName("RPC", cliRPC)
	go server.Accept(l)

	// go startSocket(&config)
	logger.Println("Ready to listen")

	// Signal ready to systemd
	systemd.SdNotify(false, "READY=1")

	// Simple polling to tell systemd that we're alive
	// Eventually this should actually check that things are working
	go func() {
		interval, err := systemd.SdWatchdogEnabled(false)
		if err != nil || interval == 0 {
			return
		}
		for {
			systemd.SdNotify(false, "WATCHDOG=1")
			time.Sleep(interval / 3)
		}
	}()

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
