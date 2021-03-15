package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"hermes/metrics"

	"github.com/lancer-kit/uwe/v2"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
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
	rabbitURL := cfg.RabbitMQ.URL()

	for i, exchange := range cfg.Exchanges {
		emCfg := emitterCfg{
			uri:          rabbitURL,
			authFormat:   cfg.AuthFormat,
			distribution: exchange,
			tickPeriod:   cfg.TickPeriod,
		}

		chief.AddWorker(
			uwe.WorkerName(fmt.Sprintf("emmiter_%d", i)),
			NewRabbitEmitter(emCfg, collector.Add),
		)
	}

	chief.Run()

	err := collector.WriteToFile("./", true)
	if err != nil {
		logger.WithError(err).Error("failed to write metrics to file")
	}
}

func getConfig() RabbitEmitterCfg {
	cfgPath := flag.String("conf", "rabbit_emitter.yaml", "path to config")
	flag.Parse()

	var cfg RabbitEmitterCfg
	yamlFile, err := ioutil.ReadFile(*cfgPath)
	if err != nil {
		log.Fatalf("can`t read confg file: %s", err)
	}

	err = yaml.Unmarshal(yamlFile, &cfg)
	if err != nil {
		log.Fatalf("can`t unmarshal the config file: %s", err)
	}
	return cfg
}
