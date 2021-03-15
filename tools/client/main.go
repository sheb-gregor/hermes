package main

import (
	"fmt"

	"hermes/metrics"

	"github.com/lancer-kit/uwe/v2"
	"github.com/sirupsen/logrus"
)

type metricsCollector struct{ *metrics.SafeMetrics }

func (m *metricsCollector) Init() error               { return nil }
func (m *metricsCollector) Run(ctx uwe.Context) error { m.SafeMetrics.Collect(ctx); return nil }

func main() {
	cfg := getConfig()
	var collector metricsCollector
	collector.SafeMetrics = collector.SafeMetrics.New()

	logger := logrus.New().WithField("app", "hermes-client")
	logger.Logger.Level = logrus.InfoLevel

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

	chief.AddWorker("metrics_collector", &collector)

	addWorker := func(behavior string, id int) {
		name := fmt.Sprintf("%s_%d", behavior, id)
		token := fmt.Sprintf("%s_%d", "user", id)

		chief.AddWorker(uwe.WorkerName(name),
			NewActor(
				cfg.HermesURL,
				ActorOpts{
					Behavior:       behavior,
					Token:          token,
					Role:           cfg.ActorParams.Role,
					Origin:         cfg.ActorParams.Origin,
					DropDelay:      cfg.ActorParams.DropDelay,
					ReconnectDelay: cfg.ActorParams.ReconnectDelay,
				},
				logger.WithField("worker", name),
				collector.Add,
			))
	}

	normalAnon := cfg.Behaviors.NormalAnon * cfg.Connections / 100
	for i := 0; i < normalAnon; i++ {
		addWorker(BehaviorNormalAnon, i)
	}

	normalUser := cfg.Behaviors.NormalUser * cfg.Connections / 100
	for i := 0; i < normalUser; i++ {
		addWorker(BehaviorNormalUser, i)
	}

	reconnectAnon := cfg.Behaviors.ReconnectAnon * cfg.Connections / 100
	for i := 0; i < reconnectAnon; i++ {
		addWorker(BehaviorReconnectAnon, i)
	}

	reconnectUser := cfg.Behaviors.ReconnectUser * cfg.Connections / 100
	for i := 0; i < reconnectUser; i++ {
		addWorker(BehaviorReconnectUser, i)
	}

	drop := cfg.Behaviors.Drop * cfg.Connections / 100
	for i := 0; i < drop; i++ {
		addWorker(BehaviorMonkey, i)
	}

	chief.Run()

	err := collector.WriteToFile("./", true)
	if err != nil {
		logger.WithError(err).Error("failed to write metrics to file")
	}
}
