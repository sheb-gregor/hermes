package main

import (
	"errors"
	"flag"
	"io/ioutil"
	"log"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"gopkg.in/yaml.v2"
)

type ClientCfg struct {
	HermesURL   string               `json:"hermes_url" yaml:"hermes_url"`
	Connections int                  `json:"connections" yaml:"connections"`
	ActorParams ActorConfig          `json:"actors_params" yaml:"actors_params"`
	Behaviors   BehaviorDistribution `json:"behaviors" yaml:"behaviors"`
}

func (cfg ClientCfg) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.HermesURL, validation.Required, is.URL),
		validation.Field(&cfg.ActorParams, validation.Required),
		validation.Field(&cfg.Behaviors, validation.Required),
	)
}

type BehaviorDistribution struct {
	NormalAnon    int `json:"normal_anon" yaml:"normal_anon"`
	NormalUser    int `json:"normal_user" yaml:"normal_user"`
	ReconnectAnon int `json:"reconnect_anon" yaml:"reconnect_anon"`
	ReconnectUser int `json:"reconnect_user" yaml:"reconnect_user"`
	Drop          int `json:"drop" yaml:"drop"`
}

func (cfg BehaviorDistribution) Validate() error {
	sum := cfg.NormalAnon + cfg.NormalUser + cfg.ReconnectAnon + cfg.ReconnectUser + cfg.Drop
	if sum != 100 {
		return errors.New("sum of behaviors must be equal 100")
	}

	return nil
}

type ActorConfig struct {
	Origin         string `json:"origin" yaml:"origin"`
	Role           string `json:"role" yaml:"role"`
	DropDelay      int    `json:"drop_delay" yaml:"drop_delay"`           // in ms
	ReconnectDelay int    `json:"reconnect_delay" yaml:"reconnect_delay"` // in ms
}

func (cfg ActorConfig) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Origin, validation.Required),
		validation.Field(&cfg.Role, validation.Required),
		validation.Field(&cfg.DropDelay, validation.Required),
		validation.Field(&cfg.ReconnectDelay, validation.Required),
	)
}

func getConfig() ClientCfg {
	path := flag.String("conf", "client.yaml", "path to config file")
	flag.Parse()
	var cfg ClientCfg

	yamlFile, err := ioutil.ReadFile(*path)
	if err != nil {
		log.Fatalf("can`t read confg file: %s", err)
	}
	err = yaml.Unmarshal(yamlFile, &cfg)
	if err != nil {
		log.Fatalf("can`t unmarshal the config file: %s", err)
	}
	return cfg
}
