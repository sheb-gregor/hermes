package models

import "encoding/json"

const (
	ActionAddQueue = 1
	ActionRmQueue  = -1
)

type ManageQueue struct {
	Queue  string
	Action int
}

type MessageMeta struct {
	Broadcast bool   `json:"-"`
	Role      string `json:"-"`
	TTL       int64  `json:"-"`
	UserUID   string `json:"-"`
}

type Message struct {
	Meta MessageMeta `json:"-"`

	Channel string `json:"channel"`
	Event   string `json:"event"`

	Command map[string]string      `json:"command,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

type ShortMessage struct {
	Event string                 `json:"event"`
	Data  map[string]interface{} `json:"data,omitempty"`
}

func (msg Message) ToShort() ShortMessage {
	val, eventData := msg.Data[msg.Event]
	m := ShortMessage{
		Event: msg.Event,
	}

	stringVal, str := val.(string)
	if eventData && str {
		m.Data = map[string]interface{}{msg.Event: json.RawMessage(stringVal)}
		return m
	}

	m.Data = make(map[string]interface{}, len(msg.Data))
	for k := range msg.Data {
		m.Data[k] = msg.Data[k]
	}

	return m
}
