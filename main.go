package main

import (
	"fmt"
	"os"

	"hermes/app"
	"hermes/config"
	"hermes/log"

	"github.com/urfave/cli"
)

const flagConfig = "config"

//nolint:gochecknoglobals
var (
	build   = "n/a"
	version = "n/a"
)

func main() {
	config.App.Build = build
	config.App.Version = version

	newApp := cli.NewApp()
	newApp.Usage = "A " + config.ServiceName + " service"
	newApp.Version = config.App.Version + ":" + config.App.Build
	newApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  flagConfig + ", c",
			Value: "./config.yaml",
		},
	}
	newApp.Commands = []cli.Command{
		{
			Name:   "serve",
			Usage:  "starts " + config.ServiceName + " workers",
			Action: serveAction,
		},
	}

	if err := newApp.Run(os.Args); err != nil {
		fmt.Println("failed run newApp:", err.Error())
		os.Exit(1)
	}
}

func serveAction(c *cli.Context) error {
	cfg, err := config.ReadConfig(c.GlobalString(flagConfig))
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	logger := log.New(cfg.Log)

	chief := app.InitChief(logger, cfg)
	chief.Run()
	return nil
}
