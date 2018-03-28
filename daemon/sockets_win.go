// +build windows

package daemon

import (
	"net"
	"os"

	"github.com/Microsoft/go-winio"
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
	logger.Println("Removing socket:", windowsPipe)
	os.Remove(windowsPipe)
}
