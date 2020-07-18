package config

import (
	"io/ioutil"

	"hermes/metrics"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/lancer-kit/armory/log"
	"github.com/lancer-kit/noble"
	"github.com/lancer-kit/uwe/v2/presets/api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const (
	ServiceName = "hermes"
)

// Cfg main structure of the app configuration.
type Cfg struct {
	Log      LogConfig  `json:"log" yaml:"log"`
	API      api.Config `json:"api" yaml:"api"`
	RabbitMQ RabbitMQ   `json:"rabbit_mq" yaml:"rabbit_mq"`

	EnableAuth bool `json:"enable_auth" yaml:"enable_auth"`
	EnableUI   bool `json:"enable_ui" yaml:"enable_ui"`

	AuthorizedServices map[string]noble.Secret `json:"authorized_services" yaml:"authorized_services"`
	AuthProviders      map[string]AuthProvider `json:"auth_providers" yaml:"auth_providers"`
	Cache              CacheCfg                `json:"cache" yaml:"cache"`

	Monitoring metrics.MonitoringConf `json:"monitoring" yaml:"monitoring"`
}

func (cfg Cfg) Validate() error {
	if cfg.EnableAuth {
		err := validation.Required.Validate(cfg.AuthProviders)
		if err != nil {
			return errors.Wrap(err, "auth_providers")
		}

		for name, provider := range cfg.AuthProviders {
			err = provider.Validate()
			if err != nil {
				return errors.Wrap(err, name)
			}
		}
		err = validation.Required.Validate(cfg.AuthorizedServices)
		if err != nil {
			return errors.Wrap(err, "authorized_services")
		}

		for service, secret := range cfg.AuthorizedServices {
			err = noble.RequiredSecret.Validate(secret)
			if err != nil {
				return errors.Wrap(err, service)
			}
		}
	}

	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.API, validation.Required),
		validation.Field(&cfg.Log, validation.Required),
		validation.Field(&cfg.RabbitMQ, validation.Required),
		validation.Field(&cfg.Cache, validation.Required),
		validation.Field(&cfg.Monitoring, validation.Required),
	)
}

func ReadConfig(path string) Cfg {
	rawConfig, err := ioutil.ReadFile(path)
	if err != nil {
		logrus.New().WithError(err).
			WithField("path", path).
			Fatal("unable to read config file")
	}

	config := new(Cfg)
	err = yaml.Unmarshal(rawConfig, config)
	if err != nil {
		logrus.New().WithError(err).
			WithField("raw_config", rawConfig).
			Fatal("unable to unmarshal config file")
	}

	err = config.Validate()
	if err != nil {
		logrus.New().WithError(err).
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

	// need to sanitize allowed roles
	for origin, provider := range config.AuthProviders {
		provider.Init()
		config.AuthProviders[origin] = provider
	}

	if config.Monitoring.Metrics {
		metrics.Init(metrics.CollectorOpts{})
		registerAllKeys()
	}

	return *config
}

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
