package socket

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/lancer-kit/uwe/v2"
	"github.com/sirupsen/logrus"
	"gitlab.inn4science.com/ctp/hermes/models"
)

// var MetricsCollector *metrics.SafeMetrics

// Hub maintains the set of active sessionStorage and broadcasts MessagesChan to the connected clients.
type Hub struct {
	log    *logrus.Entry
	ctx    context.Context
	cancel context.CancelFunc

	eventStream     EventStream
	sessionStorage  wsSessionStorage
	usersSessions   wsUsersSessions
	liveConnections wsLiveSessions
}

func NewHub(logger *logrus.Entry) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	return &Hub{
		log:    logger.WithField("sub_service", "ws-hub"),
		ctx:    ctx,
		cancel: cancel,

		sessionStorage:  wsSessionStorage{new(sync.Map)},
		usersSessions:   wsUsersSessions{new(sync.Map)},
		liveConnections: wsLiveSessions{new(sync.Map)},
		eventStream:     make(EventStream, 256),
	}
}

func (h *Hub) EventBus() EventStream {
	return h.eventStream
}

func (h *Hub) Context() context.Context {
	return h.ctx
}

func (h *Hub) addSession(client *Session) {
	// MetricsCollector.Add("hub.add_client")

	h.sessionStorage.Store(client.connUID, client)
	h.usersSessions.addSessionID(client.userUUID, client.connUID)
	h.liveConnections.Store(client.connUID, true)

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writeToStream()
	go client.readStream()
}

func (h *Hub) rmSession(sessionID int64) {
	client, err := h.sessionStorage.getByID(sessionID)
	if err != nil {
		h.log.WithError(err).
			WithField("connUID", sessionID).
			Warn("unable to get client by connUID")
		return
	}

	h.sessionStorage.Delete(sessionID)
	h.usersSessions.rmSessionID(client.userUUID, sessionID)
	h.liveConnections.Delete(sessionID)

	close(client.send)

	// MetricsCollector.Add("hub.rm_client")
	if err = client.conn.Close(); err != nil {
		h.log.WithError(err).
			WithField("connUID", sessionID).
			Error("failed to remove client")
	}
}

func (h Hub) GetSessionsCount() int64 {
	var counter int64
	h.sessionStorage.Range(func(key interface{}, value interface{}) bool {
		counter++
		return true
	})

	return counter
}

func (h *Hub) Init() error {
	return nil
}

func (h *Hub) Run(wCtx uwe.Context) error {
	gcTicker := time.NewTicker(2 * time.Hour)

	for {
		select {
		case <-gcTicker.C:
			h.log.Info("force garbage collection running...")
			runtime.GC()

		case event := <-h.eventStream:
			switch event.Kind {
			case EKNewSession:
				h.addSession(event.Session)

			case EKHandshake:
				// MetricsCollector.Add("hub.HandshakesChan")
				client, err := h.sessionStorage.getByID(event.SessionID)
				if err != nil {
					h.log.WithError(err).Warn("unable to get client by SessionID")
					h.rmSession(event.SessionID)
					continue
				}

				client.send <- &models.Message{Event: EvHandshake, Channel: EvStatusChannel}
				h.log.Debug("Response sent to ", client.connUID)

			case EKMessage:
				// MetricsCollector.Add("hub.MessagesChan")
				if event.Message.Broadcast {
					h.broadCastAll(event.Message)
				} else {
					h.sendDirect(event.Message)
				}

			case EKUnregister:
				// MetricsCollector.Add("hub.UnregisterChan")
				h.rmSession(event.SessionID)
			}

		case <-wCtx.Done():
			gcTicker.Stop()

			h.cancel()
			h.closeSockets()

			return nil
		}

	}
}

func (h Hub) closeSockets() {
	h.sessionStorage.Range(func(key interface{}, value interface{}) bool {
		uid, ok := key.(int64)
		if !ok {
			h.log.WithField("status", ok).Error("unable to cast value to connUID")
			return false
		}

		h.rmSession(uid)

		return true
	})
}

func (h Hub) broadCastAll(message *models.Message) {
	h.sessionStorage.Range(func(key interface{}, value interface{}) bool {
		client, ok := value.(*Session)
		if !ok {
			h.log.WithField("status", ok).Error("unable to cast value to client")
			return false
		}

		client.send <- message
		return true
	})
}

func (h Hub) sendDirect(message *models.Message) {
	for sessionID := range h.usersSessions.getSessions(message.UserUID) {
		session, err := h.sessionStorage.getByID(sessionID)
		if err != nil {
			h.log.WithError(err).Error("unable to get session by id")
			continue
		}

		session.send <- message
	}
}
