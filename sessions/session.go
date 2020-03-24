package sessions

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

const (
	AuthHeader = "Sec-WebSocket-Protocol"
)

type Session struct {
	ID             int64  `db:"id" json:"id"`
	Token          string `db:"token" json:"token"`
	RefreshToken   string `db:"refresh_token" json:"refreshToken"`
	UserID         string `db:"user_id" json:"userId"`
	DeviceInfo     string `db:"device_info" json:"deviceInfo"`
	ExpirationTime int64  `db:"expiration_time" json:"expirationTime"`
	Active         bool   `db:"active" json:"active"`
	CreatedAt      int64  `db:"created_at" json:"createdAt"`
	UpdatedAt      int64  `db:"updated_at" json:"updatedAt"`
}

func (s Session) Validate() error {
	return validation.ValidateStruct(s,
		validation.Field(&s.ExpirationTime, validation.Required),
		validation.Field(&s.UserID, validation.Required),
		validation.Field(&s.Active, validation.Required),
	)
}
