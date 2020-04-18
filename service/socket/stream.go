package socket

import (
	"gitlab.inn4science.com/ctp/hermes/models"
)

type EventStream chan *Event

// // this context should be passed to each session
// // and will signal that the session should be closed
// ctx context.Context
// NewSessionsChan chan *Session        // new incoming connections
// MessagesChan    chan *models.Message // inbound MessagesChan from the connected client.
// HandshakesChan  chan int64           // HandshakesChan from session with passed id
// UnregisterChan  chan int64           // need remove session with passed id

type EventKind int

const (
	EKNewSession EventKind = 1 + iota
	EKHandshake
	EKAuthorize
	EKMessage
	EKUnregister
	EKCache
)

type Event struct {
	Kind        EventKind
	SessionID   int64
	Session     *Session
	SessionInfo *models.SessionInfo
	Message     *models.Message
}
