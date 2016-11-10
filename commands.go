package main

import (
	"fmt"
	"net/rpc"

	"github.com/nickrobison/backer/shared"
	"gopkg.in/urfave/cli.v1"
)

func listWatchers(c *cli.Context) error {
	logger.Println("Listing watchers")
	logger.Println("Calling RPC server")
	client, err := rpc.Dial("unix", "/tmp/backer.sock")
	if err != nil {
		logger.Fatalln(err)
	}
	defer client.Close()

	var reply = &shared.FileWatchers{}
	err = client.Call("RPC.ListWatchers", 0, &reply)
	if err != nil {
		logger.Fatalln(err)
	}
	logger.Println("Has response", reply)

	fmt.Println("Currently watching:")
	for _, watcher := range reply.Paths {
		fmt.Println(watcher)
	}
	// rpcClient.SayHello()
	return nil
}

func listObjects(c *cli.Context) error {
	return nil
}

func listObjectVersions(c *cli.Context) error {
	return nil
}
