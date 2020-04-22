package main

import (
	"os"

	"github.com/lancer-kit/armory/log"
	"github.com/urfave/cli"
	"gitlab.inn4science.com/ctp/hermes/actions"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/info"
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
