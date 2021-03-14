package main

import (
	"os"

	"hermes/app"
	"hermes/config"

	"github.com/lancer-kit/armory/log"
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
		log.Get().WithError(err).Errorln("failed run newApp")
	}
}

func serveAction(c *cli.Context) error {
	cfg := config.ReadConfig(c.GlobalString(flagConfig))
	logger := log.Get().WithField("app", config.ServiceName)

	chief := app.InitChief(logger, cfg)
	chief.Run()
	return nil
}
