package config

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

type BoltDBConfig struct {
	FilePath string `json:"file_path" yaml:"file_path"`
	Timeout  int64  `json:"timeout" yaml:"timeout"`
	ReadOnly bool   `json:"read_only" yaml:"read_only"`
}

func (cfg BoltDBConfig) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.FilePath, validation.Required),
	)
}
