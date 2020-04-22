package main

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"gitlab.inn4science.com/ctp/hermes/config"
)

type ClientCfg struct {
	HermesURL string          `json:"hermes_url" yaml:"hermes_url"`
	RabbitMQ  config.RabbitMQ `json:"rabbit_mq" yaml:"rabbit_mq"`
	Auth      ClientAuth      `json:"client_auth" yaml:"client_auth"`
}

func (cfg ClientCfg) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.HermesURL, validation.Required, is.URL),
		validation.Field(&cfg.RabbitMQ, validation.Required),
		validation.Field(&cfg.Auth, validation.Required),
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
