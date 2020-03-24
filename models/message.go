package models

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

func (msg *Message) ToShort() ShortMessage {
	return ShortMessage{Event: msg.Event, Data: msg.Data}
}
