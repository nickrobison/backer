package main

import (
	"log"
	"os"

	"github.com/nickrobison/backer/daemon"
	"gopkg.in/urfave/cli.v1"
)

var logger *log.Logger
var rpcClient *RPC

func init() {
	logger = log.New(os.Stdout, "backer:", log.Lshortfile)
}

func main() {

	app := cli.NewApp()

	app.Version = "0.0.1"

	// Flags
	app.Flags = buildFlags()
	app.Action = parseFlags

	// Commands
	app.Commands = buildCommnds()

	app.Run(os.Args)
}

func buildFlags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:  "daemon",
			Usage: "Run the backer daemon",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: "./config.json",
			Usage: "Load config from `FILE`",
		},
	}
}

func buildCommnds() []cli.Command {

	return []cli.Command{
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "List ...",
			Subcommands: []cli.Command{
				{
					Name:    "watchers",
					Aliases: []string{"w"},
					Usage:   "List registered watchers",
					Action:  listWatchers,
				},
				{
					Name:    "objects",
					Aliases: []string{"o"},
					Usage:   "List objects in S3 Bucket",
					Action:  listObjects,
				},
				{
					Name:    "versions",
					Aliases: []string{"v"},
					Usage:   "List versions for given object",
					Action:  listObjectVersions,
				},
			},
		},
	}
}

func parseFlags(c *cli.Context) error {
	if c.Bool("daemon") {
		logger.Println("Launching daemon")
		daemon.Start(c.String("config"))
	} else {
		rpcClient = &RPC{}
	}
	return nil
}

// func startRPC() {
// 	logger.Panicln("Calling RPC server")
// 	client, err := rpc.Dial("unix", "/tmp/backer.sock")
// 	if err != nil {
// 		logger.Fatalln(err)
// 	}

// 	rpcClient = &RPC{
// 		client: client,
// 	}
// }
