package actions

import (
	"github.com/lancer-kit/armory/log"
	"github.com/urfave/cli"
	"gitlab.inn4science.com/ctp/hermes/app"
	"gitlab.inn4science.com/ctp/hermes/config"
)

func serveAction(c *cli.Context) error {
	cfg := config.ReadConfig(c.GlobalString(FlagConfig))
	logger := log.Get().WithField("app", config.ServiceName)

	chief := app.InitChief(logger, cfg)
	chief.Run()
	return nil
}
