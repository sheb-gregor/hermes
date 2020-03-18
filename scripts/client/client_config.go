package main

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/lancer-kit/uwe/v2/presets/api"
	"gitlab.inn4science.com/ctp/hermes/config"
)

type ClientCfg struct {
	API        api.Config      `json:"api" yaml:"api"`
	RabbitMQ   config.RabbitMQ `json:"rabbit_mq" yaml:"rabbit_mq"`
	ConnNumber ConnCfg         `json:"conn_number" yaml:"conn_number"`
}

func (cfg ClientCfg) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.API, validation.Required),
		validation.Field(&cfg.RabbitMQ, validation.Required),
		validation.Field(&cfg.ConnNumber, validation.Required),
	)
}

type ConnCfg struct {
	Conn     int64 `json:"conn" yaml:"conn"`
	Stable   int64 `json:"stable" yaml:"stable"`
	Timeout  int64 `json:"timeout" yaml:"timeout"`
	Unstable int64 `json:"unstable" yaml:"unstable"`
}

func (cfg ConnCfg) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Conn, validation.Required),
		validation.Field(&cfg.Stable, validation.Required),
		validation.Field(&cfg.Timeout, validation.Required),
		validation.Field(&cfg.Unstable, validation.Required),
	)
}
