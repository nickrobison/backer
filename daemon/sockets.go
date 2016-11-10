package daemon

import (
	"net"
	"net/rpc"
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

func StartSocket(c *backerConfig) {
	logger.Println("Listening on unix socket:", unixSocket)
	l, err := net.Listen("unix", unixSocket)
	if err != nil {
		logger.Fatalln(err)
	}

	cliRPC := &RPC{
		Config: c,
	}
	server := rpc.NewServer()
	server.RegisterName("RPC", cliRPC)
	server.Accept(l)
	// for {
	//     fd, err := l.Accept()
	//     if err != nil {
	//         logger.Fatalln(err)
	//         return
	//     }
	//     dataHandler(fd)
	//     fd.Close()
	// }
}

func removeSocket() {
	logger.Println("Removing socket:", unixSocket)
	os.Remove(unixSocket)
}
