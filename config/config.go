package config

import (
	"io/ioutil"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/lancer-kit/armory/log"
	"github.com/lancer-kit/noble"
	"github.com/lancer-kit/uwe/v2/presets/api"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const ServiceName = "hermes"

// Config is a options for the initialization
// of the default logrus.Entry.
type LogConfig struct {
	// AppName identifier of the app.
	AppName string `json:"app_name" yaml:"app_name"`
	// Level is a string representation of the `lorgus.Level`.
	Level noble.Secret `json:"level" yaml:"level"`
	// Sentry is a DSN string for sentry hook.
	Sentry string `json:"sentry" yaml:"sentry"`
	// AddTrace enable adding of the filename field into log.
	AddTrace bool `json:"add_trace" yaml:"add_trace"`
	// JSON enable json formatted output.
	JSON bool `json:"json" yaml:"json"`
}

func (cfg LogConfig) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.AppName, validation.Required),
		validation.Field(&cfg.Level, validation.Required, noble.RequiredSecret),
	)
}

// Cfg main structure of the app configuration.
type Cfg struct {
	Log LogConfig `json:"log" yaml:"log"`

	API      api.Config `json:"api" yaml:"api"`
	RabbitMQ RabbitMQ   `json:"rabbit_mq" yaml:"rabbit_mq"`
}

func (cfg Cfg) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.API, validation.Required),
		validation.Field(&cfg.Log, validation.Required),
		validation.Field(&cfg.RabbitMQ, validation.Required),
	)
}

func ReadConfig(path string) Cfg {
	rawConfig, err := ioutil.ReadFile(path)
	if err != nil {
		logrus.New().
			WithError(err).
			WithField("path", path).
			Fatal("unable to read config file")
	}

	config := new(Cfg)
	err = yaml.Unmarshal(rawConfig, config)
	if err != nil {
		logrus.New().
			WithError(err).
			WithField("raw_config", rawConfig).
			Fatal("unable to unmarshal config file")
	}

	err = config.Validate()
	if err != nil {
		logrus.New().
			WithError(err).
			Fatal("Invalid configuration")
	}

	_, err = log.Init(log.Config{
		AppName:  config.Log.AppName,
		Level:    config.Log.Level.Get(),
		Sentry:   config.Log.Sentry,
		AddTrace: config.Log.AddTrace,
		JSON:     config.Log.JSON,
	})
	if err != nil {
		logrus.New().
			WithError(err).
			Fatal("Unable to init log")
	}
	return *config
}
