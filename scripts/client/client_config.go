package main

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/lancer-kit/uwe/v2/presets/api"
	"gitlab.inn4science.com/ctp/hermes/config"
)

const (
	ClientMetricsBucket = "clientMetrics"
)

type ClientCfg struct {
	API      api.Config       `json:"api" yaml:"api"`
	RabbitMQ config.RabbitMQ  `json:"rabbit_mq" yaml:"rabbit_mq"`
	Auth     ClientAuth       `json:"client_auth" yaml:"client_auth"`
	Metrics  config.NutsDBCfg `json:"metrics" yaml:"metrics"`
}

func (cfg ClientCfg) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.API, validation.Required),
		validation.Field(&cfg.RabbitMQ, validation.Required),
		validation.Field(&cfg.Auth, validation.Required),
		validation.Field(&cfg.Metrics, validation.Required),
	)
}

type ClientAuth struct {
	Token  string `json:"token"`
	Origin string `json:"origin"`
	Role   string `json:"role"`
}

func (cfg ClientAuth) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Token, validation.Required),
		validation.Field(&cfg.Origin, validation.Required),
		validation.Field(&cfg.Role, validation.Required),
	)
}
