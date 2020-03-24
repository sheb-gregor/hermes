package models

import (
	"errors"

	"github.com/streadway/amqp"
)

const (
	RHeaderUUID       = "uuid"
	RHeaderVisibility = "visibility"
	RHeaderEvent      = "event_type"
	RHeaderRole       = "role"
	RHeaderCacheTTL   = "cache_ttl"

	VisibilityBroadcast = "broadcast"
	VisibilityDirect    = "direct"
	VisibilityInternal  = "internal"

	VRoleAny = "any"
)

type RabbitHeader struct {
	Visibility string
	UUID       string
	Event      string
	Role       string
	CacheTTL   int64
	Broadcast  bool
}

func ParseRabbitHeader(message amqp.Delivery) (RabbitHeader, error) {
	var ok bool
	var params RabbitHeader

	params.Visibility, ok = message.Headers[RHeaderVisibility].(string)
	if !ok {
		return params, errors.New("message don't have RHeaderVisibility")
	}

	if params.Visibility == VisibilityInternal {
		return params, nil
	}

	params.Broadcast = params.Visibility == VisibilityBroadcast

	params.UUID, ok = message.Headers[RHeaderUUID].(string)
	if !ok && params.Visibility == VisibilityDirect {
		return params, errors.New("message don't have RHeaderUUID")
	}

	params.Event, ok = message.Headers[RHeaderEvent].(string)
	if !ok {
		return params, errors.New("message don't have RHeaderEvent")
	}

	params.Role, ok = message.Headers[RHeaderRole].(string)
	if !ok || params.Role == "" {
		params.Role = VRoleAny
	}

	// empty cache ttl means — do not cache event
	params.CacheTTL, _ = message.Headers[RHeaderCacheTTL].(int64)
	return params, nil
}
