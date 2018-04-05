package daemon

import (
	"encoding/json"
	"io/ioutil"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	systemd "github.com/coreos/go-systemd/daemon"
	"github.com/fsnotify/fsnotify"
	"github.com/nickrobison/backer/backends"
	"github.com/nickrobison/backer/shared"
)

// var logger *log.Logger

// func init() {
// 	logger = log.New(os.Stdout, "backer-daemon:", log.Lshortfile)
// }

// Start backer daemon
// Registers an S3 uploader and a fileManager to watch the watchers
func Start(configLocation string) {
	log.Println("Starting up Backer daemon")

	// Read in the config file
	file, err := ioutil.ReadFile(configLocation)
	if err != nil {
		log.Fatalln("Cannot read config file:", configLocation)
	}

	var config shared.BackerConfig
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatalln(err)
	}

	// Validate the paths
	err = config.ValidateWatcherPaths()
	if err != nil {
		log.Fatalln(err)
	}

	// Register the shutdown handler
	done := make(chan bool)
	go shutdown(done)

	// Create a new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalln(err)
	}
	defer watcher.Close()

	// Create new uploader
	var clients []backends.Uploader

	s3Client := backends.NewS3Uploader(&config.S3)
	clients = append(clients, s3Client)
	config.Backends = clients

	// Register new file manager
	fm := NewFileManager(&config)

	// Register all watchers
	for _, newWatcher := range config.Watchers {
		path, err := newWatcher.GetPath()
		if err != nil {
			log.Fatalln(err)
		}
		fm.RegisterWatcherPath(path, newWatcher.BucketPath)
		err = watcher.Add(path)
		if err != nil {
			log.Fatalln(err)
		}
	}
	fm.Start(watcher.Events, watcher.Errors)

	// Start listener
	l, err := getSocket()
	if err != nil {
		log.Fatalln(err)
	}
	defer l.Close()

	cliRPC := &RPC{
		Config: &config,
	}
	server := rpc.NewServer()
	server.RegisterName("RPC", cliRPC)
	go server.Accept(l)

	// go startSocket(&config)
	log.Debugln("Ready to listen")

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
	log.Println("Shutting down")
	removeSocket()
	// Cleanup code
	done <- true
}
