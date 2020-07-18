package main

import (
	"os"

	"hermes/actions"
	"hermes/config"
	"hermes/info"

	"github.com/lancer-kit/armory/log"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Usage = "A " + config.ServiceName + " service"
	app.Version = info.App.Version + ":" + info.App.Build
	app.Flags = actions.GetFlags()
	app.Commands = actions.GetCommands()

	if err := app.Run(os.Args); err != nil {
		log.Get().WithError(err).Errorln("failed run app")
	}
}
