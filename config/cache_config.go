package config

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

const (
	StorageTypeRedis  = "redis"
	StorageTypeNutsDB = "nutsdb"
)

type CacheCfg struct {
	Disable bool      `json:"disable" yaml:"disable"`
	Type    string    `json:"type" yaml:"type"`
	Redis   RedisConf `json:"redis" yaml:"redis"`
	NutsDB  NutsDBCfg `json:"nutsdb" yaml:"nutsdb"`
}

func (cfg CacheCfg) Validate() error {
	if cfg.Disable {
		return nil
	}

	validators := []*validation.FieldRules{
		validation.Field(&cfg.Type, validation.Required, validation.In(StorageTypeRedis, StorageTypeNutsDB)),
	}

	switch cfg.Type {
	case StorageTypeNutsDB:
		validators = append(validators, validation.Field(&cfg.NutsDB, validation.Required))
	case StorageTypeRedis:
		validators = append(validators, validation.Field(&cfg.Redis, validation.Required))
	}
	return validation.ValidateStruct(&cfg, validators...)
}
