package config

import (
	"fmt"
	"os"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/lancer-kit/noble"
)

type RabbitMQ struct {
	Auth   RabbitAuth `json:"auth" yaml:"auth"`
	Common Exchange   `json:"common" yaml:"common"`
	Subs   []Exchange `json:"subs" yaml:"subs"`
}

func (cfg RabbitMQ) GetCommonSub(queue string) Exchange {
	return Exchange{
		Queue:        queue,
		Exchange:     cfg.Common.Exchange,
		ExchangeType: cfg.Common.ExchangeType,
		RoutingKey:   cfg.Common.Queue,
	}
}

func (cfg RabbitMQ) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Auth, validation.Required),
		validation.Field(&cfg.Subs, validation.Required),
		validation.Field(&cfg.Common, validation.Required),
	)
}

type RabbitAuth struct {
	Host            string       `json:"host" yaml:"host"`
	User            noble.Secret `json:"user" yaml:"user"`
	Password        noble.Secret `json:"password" yaml:"password"`
	ConsumerTag     string       `json:"consumer_tag" yaml:"consumer_tag"`
	CreateExchanges bool         `json:"create_exchanges" yaml:"create_exchanges"`
	StrictMode      bool         `json:"strict_mode" yaml:"strict_mode"`
}

func (cfg RabbitAuth) URL() string {
	return fmt.Sprintf("amqp://%s:%s@%s", cfg.User.Get(), cfg.Password.Get(), cfg.Host)
}

func (cfg RabbitAuth) GetConsumerTag(queue string) string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s:%s_%s", hostname, queue, cfg.ConsumerTag)
}

type Exchange struct {
	Exchange     string `json:"exchange" yaml:"exchange"`
	ExchangeType string `json:"exchange_type" yaml:"exchange_type"`
	RoutingKey   string `json:"routing_key" yaml:"routing_key"`
	Queue        string `json:"queue" yaml:"queue"`
	// Durable exchanges will survive server restarts
	Durable bool `json:"durable" yaml:"durable"`
	// Will remain declared when there are no remaining bindings.
	AutoDelete bool `json:"auto_delete" yaml:"auto_delete"`
	// When noWait is true, declare without waiting for a confirmation from the server.
	NoWait bool `json:"no_wait" yaml:"no_wait"`
}

func (cfg Exchange) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Exchange, validation.Required),
		validation.Field(&cfg.ExchangeType, validation.Required),
	)
}
