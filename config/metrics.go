package config

import (
	"gitlab.inn4science.com/ctp/hermes/metrics"
)

const (
	WSActiveSessions       metrics.MKey = "ws_active_sessions"
	WSAuthorizedSessions   metrics.MKey = "ws_authorized_sessions"
	WSClosedSessions       metrics.MKey = "ws_closed_sessions"
	IncomingRabbitMessages metrics.MKey = "incoming_rabbit_messages"
	SentBroadcastMessages  metrics.MKey = "sent_broadcast_messages"
	SentDirectMessages     metrics.MKey = "sent_direct_messages"
	DroppedMessages        metrics.MKey = "dropped_messages"
	DeliveredMessages      metrics.MKey = "delivered_messages"
)

func registerAllKeys() {
	metrics.RegisterGauges(
		WSActiveSessions,
		WSAuthorizedSessions,
		WSClosedSessions,
		IncomingRabbitMessages,
		SentBroadcastMessages,
		SentDirectMessages,
		DroppedMessages,
		DeliveredMessages,
	)
}
