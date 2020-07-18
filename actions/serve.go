package actions

import (
	"hermes/app"
	"hermes/config"

	"github.com/lancer-kit/armory/log"
	"github.com/urfave/cli"
)

func serveAction(c *cli.Context) error {
	cfg := config.ReadConfig(c.GlobalString(FlagConfig))
	logger := log.Get().WithField("app", config.ServiceName)

	chief := app.InitChief(logger, cfg)
	chief.Run()
	return nil
}
