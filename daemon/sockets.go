// +build !windows

package daemon

import (
	"net"
	"os"

	log "github.com/sirupsen/logrus"
)

const unixSocket = "/tmp/backer.sock"

func getSocket() (net.Listener, error) {

	if fileExists(unixSocket) {
		log.Warnf("File: %s already, exists, application may have crashed.\n", unixSocket)
		err := os.Remove(unixSocket)
		if err != nil {
			return nil, err
		}
	}

	l, err := net.Listen("unix", unixSocket)
	if err != nil {
		return nil, err
	}
	// Open up permissions
	os.Chmod(unixSocket, 0777)
	return l, nil
}

func removeSocket() {
	log.Debugln("Removing socket:", unixSocket)
	os.Remove(unixSocket)
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}
