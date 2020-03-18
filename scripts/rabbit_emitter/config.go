package main

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"gitlab.inn4science.com/ctp/hermes/config"
)

type RabbitEmitterCfg struct {
	RabbitMQ   config.RabbitMQ `json:"rabbit_mq" yaml:"rabbit_mq"`
	ConnNumber ConnCfg         `json:"conn_number" yaml:"conn_number"`
}

func (cfg RabbitEmitterCfg) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.RabbitMQ, validation.Required),
		validation.Field(&cfg.ConnNumber, validation.Required),
	)
}

type ConnCfg struct {
	ConnPercentage int `json:"conn_percentage" yaml:"conn_percentage"`
}

func (cfg ConnCfg) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.ConnPercentage, validation.Required),
	)
}
