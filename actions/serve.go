package actions

import (
	"github.com/lancer-kit/armory/log"
	"github.com/urfave/cli"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/service"
)

func serveAction(c *cli.Context) error {
	cfg := config.ReadConfig(c.GlobalString(FlagConfig))
	logger := log.Get().WithField("app", config.ServiceName)

	chief := service.InitChief(logger, cfg)
	chief.Run()
	return nil
}
