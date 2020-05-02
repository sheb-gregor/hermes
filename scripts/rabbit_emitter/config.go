package main

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"gitlab.inn4science.com/ctp/hermes/config"
)

type RabbitEmitterCfg struct {
	RabbitMQ   config.RabbitAuth `json:"rabbit_mq" yaml:"rabbit_mq"`
	AuthFormat TokenFormat       `json:"auth_format" yaml:"auth_format"`
	Exchanges  []Distribution    `json:"exchanges" yaml:"exchanges"`
	TickPeriod uint              `json:"tick_period" yaml:"tick_period"`
}

func (cfg RabbitEmitterCfg) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.RabbitMQ, validation.Required),
		validation.Field(&cfg.AuthFormat, validation.Required),
		validation.Field(&cfg.Exchanges, validation.Required),
		validation.Field(&cfg.TickPeriod, validation.Required),
	)
}

type Distribution struct {
	Exchange     string `json:"exchange" yaml:"exchange"`
	Queue        string `json:"queue" yaml:"queue"`
	DirectRandom bool   `json:"direct_random" yaml:"direct_random"`
	Broadcast    bool   `json:"broadcast" yaml:"broadcast"`
}

func (cfg Distribution) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Exchange, validation.Required),
		validation.Field(&cfg.Queue, validation.Required),
	)
}

type TokenFormat struct {
	Format string `json:"format" yaml:"format"`
	From   uint   `json:"from" yaml:"from"`
	To     uint   `json:"to" yaml:"to"`
}

func (cfg TokenFormat) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Format, validation.Required),
		validation.Field(&cfg.From, validation.Required),
		validation.Field(&cfg.To, validation.Required),
	)
}
