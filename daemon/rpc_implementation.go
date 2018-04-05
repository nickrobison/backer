package daemon

import (
	"github.com/nickrobison/backer/shared"
	log "github.com/sirupsen/logrus"
)

// RPC - RPC interface
type RPC struct {
	Config *shared.BackerConfig
}

// SayHello - Dummy Function (to remove)
func (r *RPC) SayHello(args int, reply *string) error {
	log.Println("In rpc call")
	*reply = "Hello there!"
	return nil
}

// ListWatchers - Implementation from the interface definition
func (r *RPC) ListWatchers(args int, watchers *shared.FileWatchers) error {
	var watcherPaths = make([]string, len(r.Config.Watchers))

	for i, watcher := range r.Config.Watchers {
		path, err := watcher.GetPath()
		if err != nil {
			return err
		}
		watcherPaths[i] = path
	}

	log.Debugln("Returning watcher paths")

	watchers.Paths = watcherPaths
	return nil
}
