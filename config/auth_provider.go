package config

import (
	"hermes/models"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/lancer-kit/armory/tools"
	"github.com/lancer-kit/noble"
)

type AuthProvider struct {
	URL          tools.URL    `json:"url" yaml:"url"`
	AccessKey    noble.Secret `json:"access_key" yaml:"access_key"`
	Header       string       `json:"header" yaml:"header"`
	AllowedRoles []string     `json:"allowed_roles" yaml:"allowed_roles"`

	allowedRoles map[string]struct{}
}

func (cfg AuthProvider) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.URL, validation.Required),
		validation.Field(&cfg.Header, validation.Required),
		validation.Field(&cfg.AccessKey, validation.Required, noble.RequiredSecret),
		validation.Field(&cfg.AllowedRoles, validation.Required),
	)
}

func (cfg *AuthProvider) Init() {
	cfg.allowedRoles = map[string]struct{}{}
	if len(cfg.AllowedRoles) == 0 {
		cfg.allowedRoles[models.VRoleAny] = struct{}{}
	}

	for _, role := range cfg.AllowedRoles {
		cfg.allowedRoles[role] = struct{}{}
	}
}

func (cfg *AuthProvider) CheckRole(role string) bool {
	if cfg.allowedRoles != nil {
		return true
	}

	_, allowRole := cfg.allowedRoles[role]
	_, allowAny := cfg.allowedRoles[models.VRoleAny]
	return allowRole || allowAny

}
