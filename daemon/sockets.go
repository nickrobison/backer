// +build !windows

package daemon

import (
	"net"
	"os"
)

const unixSocket = "/tmp/backer.sock"

// func dataHandler(c net.Conn) {
//     buf := make([]byte, 512)

//     _, err := c.Read(buf)
//     if err != nil {
//         logger.Println("Error reading data:", err)
//     }
//     logger.Println(string(buf))
// }

func getSocket() (net.Listener, error) {

	if fileExists(unixSocket) {
		logger.Printf("File: %s already, exists, application may have crashed.\n", unixSocket)
		err := os.Remove(unixSocket)
		if err != nil {
			return nil, err
		}
	}

	l, err := net.Listen("unix", unixSocket)
	if err != nil {
		return nil, err
	}
	return l, nil
}

// func startSocket(c *backerConfig) {
// 	logger.Println("Listening on unix socket:", unixSocket)
// 	l, err := net.Listen("unix", unixSocket)
// 	if err != nil {
// 		logger.Fatalln(err)
// 	}

// 	cliRPC := &RPC{
// 		Config: c,
// 	}
// 	server := rpc.NewServer()
// 	server.RegisterName("RPC", cliRPC)
// 	server.Accept(l)
// }

func removeSocket() {
	logger.Println("Removing socket:", unixSocket)
	os.Remove(unixSocket)
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}
