package daemon

import "github.com/nickrobison/backer/shared"

type RPC struct {
	Config *backerConfig
}

func (r *RPC) SayHello(args int, reply *string) error {
	logger.Println("In rpc call")
	*reply = "Hello there!"
	return nil
}

func (r *RPC) ListWatchers(args int, watchers *shared.FileWatchers) error {
	var watcherPaths = make([]string, len(r.Config.Watchers))

	for i, watcher := range r.Config.Watchers {
		watcherPaths[i] = watcher.GetPath()
	}

	logger.Println("Returning watcher paths")

	watchers.Paths = watcherPaths
	return nil
}
