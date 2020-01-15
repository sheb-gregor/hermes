package actions

import (
	"github.com/lancer-kit/armory/log"
	"github.com/urfave/cli"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/ws"
)

func serveAction(c *cli.Context) error {
	cfg := config.ReadConfig(c.GlobalString(FlagConfig))
	logger := log.Get().WithField("app", config.ServiceName)

	chief := ws.InitChief(logger, cfg)
	chief.Run()
	return nil
}
