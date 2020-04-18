package service

import (
	"github.com/lancer-kit/uwe/v2"
	"github.com/sirupsen/logrus"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/service/socket"
)

const (
	WorkerHub            = "hub"
	WorkerWsAPI          = "ws_api_server"
	WorkerRabbitConsumer = "rabbit_consumer"
)

func InitChief(logger *logrus.Entry, cfg config.Cfg) uwe.Chief {
	defer func() {
		rec := recover()
		if rec != nil {
			logger.WithField("rec", rec).Fatal("caught panic")
		}

		// data, _ := socket.MetricsCollector.MarshalJSON()
		// _ = ioutil.WriteFile("metrics_report.json", data, 0644)
	}()

	chief := uwe.NewChief()
	chief.UseDefaultRecover()
	chief.SetEventHandler(func(event uwe.Event) {
		var level logrus.Level
		switch event.Level {
		case uwe.LvlFatal, uwe.LvlError:
			level = logrus.ErrorLevel
		case uwe.LvlInfo:
			level = logrus.InfoLevel
		default:
			level = logrus.WarnLevel
		}

		logger.WithFields(event.Fields).
			Log(level, event.Message)
	})

	logger = logger.WithField("app_layer", "workers")

	hub := socket.NewHub(logger.WithField("worker", WorkerHub), cfg.Cache)

	rabbitConsumer, _ := NewRabbitConsumer(
		logger.WithField("worker", WorkerRabbitConsumer), cfg.RabbitMQ, hub.EventBus())
	// hub.SetSubscriptionsAdder(qManager)

	webServer := GetServer(logger.WithField("worker", WorkerWsAPI), cfg, hub.Context(), hub.Communicator())

	chief.AddWorker(WorkerHub, hub)
	chief.AddWorker(WorkerWsAPI, webServer)
	chief.AddWorker(WorkerRabbitConsumer, rabbitConsumer)
	return chief
}
