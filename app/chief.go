package app

import (
	"hermes/app/ws"
	"hermes/config"

	"github.com/lancer-kit/uwe/v2"
	"github.com/rs/zerolog"
)

const (
	WorkerHub            = "hub"
	WorkerWsAPI          = "ws_api_server"
	WorkerRabbitConsumer = "rabbit_consumer"
)

func InitChief(logger zerolog.Logger, cfg config.Cfg) uwe.Chief {
	defer func() {
		rec := recover()
		if rec != nil {
			logger.Fatal().Interface("recover", rec).Msg("caught panic")
		}
	}()
	logger = logger.With().Str("app_layer", "workers").Logger()

	chief := uwe.NewChief()
	chief.UseDefaultRecover()
	chief.SetEventHandler(func(event uwe.Event) {
		var level zerolog.Level
		switch event.Level {
		case uwe.LvlFatal, uwe.LvlError:
			level = zerolog.ErrorLevel
		case uwe.LvlInfo:
			level = zerolog.InfoLevel
		default:
			level = zerolog.WarnLevel
		}

		logger.WithLevel(level).Fields(event.Fields).Msg(event.Message)
	})

	hub := ws.NewHub(logger.With().Str("worker", WorkerHub).Logger(), cfg.Cache)

	rabbitConsumer, _ := NewRabbitConsumer(
		logger.With().Str("worker", WorkerRabbitConsumer).Logger(),
		cfg.RabbitMQ,
		hub.EventBus(),
	)

	webServer := GetServer(
		logger.With().Str("worker", WorkerWsAPI).Logger(),
		cfg,
		hub.Context(),
		hub.Communicator(),
	)

	chief.AddWorker(WorkerHub, hub)
	chief.AddWorker(WorkerWsAPI, webServer)
	chief.AddWorker(WorkerRabbitConsumer, rabbitConsumer)

	// if cfg.Monitoring.Metrics {
	// 	chief.AddWorker("monitoring", metrics.GetMonitoringServer(cfg.Monitoring, config.App))
	// }

	return chief
}
