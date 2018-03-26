package main

import (
	"net/rpc"
	"os"

	"github.com/nickrobison/backer/shared"
	"github.com/olekukonko/tablewriter"
	"gopkg.in/urfave/cli.v1"
)

func listWatchers(c *cli.Context) error {
	logger.Println("Listing watchers")
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

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Path", "Status"})

	for _, watcher := range reply.Paths {
		table.Append([]string{watcher, "OK"})
	}
	table.Render()
	return nil
}

func listObjects(c *cli.Context) error {
	return nil
}

func listObjectVersions(c *cli.Context) error {
	return nil
}
