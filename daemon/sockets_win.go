// +build windows

package daemon

import (
	"net"
	"os"

	"github.com/Microsoft/go-winio"

	log "github.com/sirupsen/logrus"
)

const windowsPipe string = `\\.\pipe\backer`

func getSocket() (net.Listener, error) {
	l, err := winio.ListenPipe(windowsPipe, nil)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func removeSocket() {
	log.Debugln("Removing socket:", windowsPipe)
	os.Remove(windowsPipe)
}
