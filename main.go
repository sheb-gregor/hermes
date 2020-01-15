package main

import (
	"os"

	"github.com/lancer-kit/armory/log"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gitlab.inn4science.com/ctp/hermes/actions"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/info"
)

func main() {
	defer func() {
		rec := recover()
		if rec != nil {
			logrus.WithField("rec", rec).Error("caught panic")
			// data, _ := socket.MetricsCollector.MarshalJSON()
			// _ = ioutil.WriteFile("metrics_report.json", data, 0644)
		}
	}()

	app := cli.NewApp()
	app.Usage = "A " + config.ServiceName + " service"
	app.Version = info.App.Version + ":" + info.App.Build
	app.Flags = actions.GetFlags()
	app.Commands = actions.GetCommands()

	if err := app.Run(os.Args); err != nil {
		log.Default.WithError(err).Errorln("failed run app")
	}
}
