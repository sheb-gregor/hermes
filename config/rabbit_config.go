package config

import (
	"fmt"
	"os"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/lancer-kit/noble"
)

type RabbitMQ struct {
	Host        string           `json:"host" yaml:"host"`
	User        noble.Secret     `json:"user" yaml:"user"`
	Password    noble.Secret     `json:"password" yaml:"password"`
	ConsumerTag string           `json:"consumer_tag" yaml:"consumer_tag"`
	Common      MqSubscription   `json:"common" yaml:"common"`
	Subs        []MqSubscription `json:"subs" yaml:"subs"`
}

type MqSubscription struct {
	Exchange     string `json:"exchange" yaml:"exchange"`
	ExchangeType string `json:"exchange_type" yaml:"exchange_type"`
	Queue        string `json:"queue" yaml:"queue"`
	RoutingKey   string `json:"routing_key" yaml:"routing_key"`
}

func (cfg RabbitMQ) URL() string {
	return fmt.Sprintf("amqp://%s:%s@%s", cfg.User.Get(), cfg.Password.Get(), cfg.Host)
}

func (cfg RabbitMQ) GetConsumerTag(queue string) string {
	hostname, _ := os.Hostname()

	return fmt.Sprintf("%s:%s_%s", hostname, queue, cfg.ConsumerTag)
}

func (cfg RabbitMQ) GetCommonSub(queue string) MqSubscription {
	return MqSubscription{
		Queue:        queue,
		Exchange:     cfg.Common.Exchange,
		ExchangeType: cfg.Common.ExchangeType,
		RoutingKey:   cfg.Common.Queue,
	}
}

func (cfg RabbitMQ) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Host, validation.Required),
		validation.Field(&cfg.User, noble.RequiredSecret),
		validation.Field(&cfg.Password, noble.RequiredSecret),
		validation.Field(&cfg.Subs, validation.Required),
		validation.Field(&cfg.Common, validation.Required),
	)
}

func (cfg MqSubscription) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Exchange, validation.Required),
		validation.Field(&cfg.ExchangeType, validation.Required),
	)
}
