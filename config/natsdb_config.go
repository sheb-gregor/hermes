package config

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

type NutsDBCfg struct {
	Path string `json:"path" yaml:"path"`
	// NutsDB will truncate data file if the active file is larger than SegmentSize
	SegmentSize int64 `json:"segment_size" yaml:"segment_size"`
}

func (cfg NutsDBCfg) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Path, validation.Required),
	)
}
