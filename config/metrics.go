package config

import (
	"gitlab.inn4science.com/ctp/hermes/metrics"
)

const (
	WSActiveSessions       metrics.MKey = "hermes_ws_active_sessions"
	WSAuthorizedSessions   metrics.MKey = "hermes_ws_authorized_sessions"
	WSClosedSessions       metrics.MKey = "hermes_ws_closed_sessions"
	IncomingRabbitMessages metrics.MKey = "hermes_incoming_rabbit_messages"
	SentBroadcastMessages  metrics.MKey = "hermes_sent_broadcast_messages"
	SentDirectMessages     metrics.MKey = "hermes_sent_direct_messages"
	DroppedMessages        metrics.MKey = "hermes_dropped_messages"
	DeliveredMessages      metrics.MKey = "hermes_delivered_messages"
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
