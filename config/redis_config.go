package config

import (
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/lancer-kit/noble"
)

type RedisConf struct {
	DevMode       bool         `json:"dev_mode" yaml:"dev_mode"`
	MaxIdleConn   int          `json:"max_idle" yaml:"max_idle"`
	MaxActiveConn int          `json:"max_active" yaml:"max_active"`
	IdleTimeout   int64        `json:"idle_timeout" yaml:"idle_timeout"`
	PingInterval  int64        `json:"ping_interval" yaml:"ping_interval"`
	Password      noble.Secret `json:"auth" yaml:"auth"`
	Host          string       `json:"host" yaml:"host"`
}

func (cfg RedisConf) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.PingInterval, validation.Required),
		validation.Field(&cfg.Host, validation.Required),
	)
}

func (cfg RedisConf) URL() string {
	pass := ""
	if cfg.Password.Get() != "" {
		pass = ":" + cfg.Password.Get() + "@"
	}

	return fmt.Sprintf("redis://%s%s", pass, cfg.Host)
}
