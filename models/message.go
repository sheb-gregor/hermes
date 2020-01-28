package models

type Message struct {
	Broadcast bool   `json:"broadcast"`
	Channel   string `json:"channel"`
	Event     string `json:"event"`
	UserUID   string `json:"-"`

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

type ManageQueue struct {
	Queue  string
	Action int
}

const (
	ActionAddQueue = 1
	ActionRmQueue  = -1
)
