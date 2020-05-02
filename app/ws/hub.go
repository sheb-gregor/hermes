package ws

import (
	"context"
	"encoding/json"
	"runtime"
	"sync"
	"time"

	"github.com/lancer-kit/uwe/v2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.inn4science.com/ctp/hermes/cache"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/metrics"
	"gitlab.inn4science.com/ctp/hermes/models"
)

// var MetricsCollector *metrics.SafeMetrics

// Hub maintains the set of active sessionStorage and broadcasts MessagesChan to the connected clients.
type Hub struct {
	log    *logrus.Entry
	ctx    context.Context
	cancel context.CancelFunc

	// subscriptionsAdder chan<- models.ManageQueue
	cacheStorage cache.Storage

	eventStream     EventStream
	sessionStorage  wsSessionStorage
	usersSessions   *wsUsersSessions
	liveConnections wsLiveSessions
}

func NewHub(logger *logrus.Entry, cfg config.CacheCfg) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	cacheStorage, err := cache.NewStorage(cfg)
	if err != nil {
		logrus.WithError(err).Fatalf("failed to initialize cache storage")
	}

	return &Hub{
		log:          logger.WithField("sub_service", "ws-hub"),
		ctx:          ctx,
		cancel:       cancel,
		cacheStorage: cacheStorage,

		sessionStorage:  wsSessionStorage{Map: new(sync.Map)},
		liveConnections: wsLiveSessions{Map: new(sync.Map)},
		usersSessions:   &wsUsersSessions{},
		eventStream:     make(EventStream, 256),
	}
}

type HubCommunicator struct {
	EventBus    EventStream
	GetSessions func() map[string]map[int64]models.SessionInfo
}

func (h *Hub) Communicator() HubCommunicator {
	return HubCommunicator{
		EventBus:    h.eventStream,
		GetSessions: h.getSessionListByUser,
	}
}
func (h *Hub) EventBus() EventStream {
	return h.eventStream
}

func (h *Hub) Context() context.Context {
	return h.ctx
}

func (h *Hub) addSession(client *Session) {
	h.sessionStorage.Store(client.info.ID, client)
	h.liveConnections.Store(client.info.ID, true)

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writeToStream()
	go client.readStream()
	metrics.Inc(config.WSActiveSessions)
}

func (h *Hub) authorizeSession(session models.SessionInfo) {
	h.usersSessions.addSessionID(session.UserID, session)
	metrics.Inc(config.WSAuthorizedSessions)
}

func (h *Hub) rmSession(sessionID int64) {
	client, err := h.sessionStorage.getByID(sessionID)
	if err != nil {
		h.log.WithError(err).WithField("connUID", sessionID).
			Warn("unable to get client by connUID")
		return
	}

	h.sessionStorage.Delete(sessionID)
	h.liveConnections.Delete(sessionID)
	h.usersSessions.rmSessionID(client.info.UserID, sessionID)

	if err = client.Close(); err != nil {
		h.log.WithError(err).
			WithField("connUID", sessionID).
			Error("failed to close client session")
	}

	metrics.Inc(config.WSClosedSessions)
	metrics.Dec(config.WSActiveSessions)
	metrics.Dec(config.WSAuthorizedSessions)
}

func (h Hub) GetSessionsCount() int64 {
	var counter int64
	h.sessionStorage.Range(func(interface{}, interface{}) bool { counter++; return true })
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
				h.processHandshake(event.SessionID)

			case EKAuthorize:
				h.authorizeSession(*event.SessionInfo)

			case EKMessage:
				err := h.processMessage(event)
				if err != nil {
					h.log.WithError(err).Debug("failed to process message")
					continue
				}

			case EKCache:
				h.processCache(event)

			case EKUnregister:
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

func (h *Hub) closeSockets() {
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

func (h *Hub) getSessionListByUser() map[string]map[int64]models.SessionInfo {
	result := map[string]map[int64]models.SessionInfo{}

	h.usersSessions.Range(func(key interface{}, value interface{}) bool {
		userID, ok := key.(string)
		if !ok {
			h.log.WithField("status", ok).Error("unable to cast key to userID")
			return false
		}

		sessions, ok := value.(map[int64]models.SessionInfo)
		if !ok {
			h.log.WithField("status", ok).Error("unable to cast value to session list")
			return false
		}
		result[userID] = sessions
		return true
	})

	return result
}

func (h *Hub) broadCastAll(message *models.Message) {
	h.sessionStorage.Range(func(key interface{}, value interface{}) bool {
		session, ok := value.(*Session)
		if !ok {
			h.log.WithField("status", ok).Error("unable to cast value to client")
			return false
		}

		if message.Meta.Role != models.VRoleAny && session.info.Role != message.Meta.Role {
			return true
		}

		session.send <- message
		metrics.Inc(config.SentBroadcastMessages)
		return true
	})
}

func (h *Hub) sendDirect(message *models.Message) {
	for sessionID := range h.usersSessions.getSessions(message.Meta.UserUID) {
		session, err := h.sessionStorage.getByID(sessionID)
		if err != nil {
			h.log.WithError(err).Error("unable to get session by id")
			continue
		}

		if message.Meta.Role != models.VRoleAny && session.info.Role != message.Meta.Role {
			h.log.WithError(err).WithFields(logrus.Fields{
				"event_role":   message.Meta.Role,
				"session_role": session.info.Role,
			}).Error("user role mismatch with required by event")
			return
		}

		session.send <- message
		metrics.Inc(config.SentDirectMessages)
	}
}

func (h *Hub) processMessage(event *Event) error {
	msg, err := json.Marshal(event.Message)
	if err != nil {
		return errors.Wrap(err, "failed to marshal message data")
	}
	if event.Message.Meta.Broadcast {
		h.broadCastAll(event.Message)
		err := h.cacheStorage.Save(cache.BroadcastBucket, nil, msg, event.Message.Meta.TTL)
		if err != nil {
			return errors.Wrap(err, "failed to cache the broadcast msg")
		}
	} else {
		h.sendDirect(event.Message)
		err := h.cacheStorage.Save(event.Message.Meta.UserUID, nil, msg, event.Message.Meta.TTL)
		if err != nil {
			return errors.Wrap(err, "failed to cache the direct msg")
		}
	}

	return nil
}

func (h *Hub) processHandshake(sessionID int64) {
	client, err := h.sessionStorage.getByID(sessionID)
	if err != nil {
		h.log.WithError(err).Warn("unable to get client by SessionID")
		h.rmSession(sessionID)
		return
	}
	client.send <- &models.Message{Event: EvHandshake, Channel: EvStatusChannel}
	h.log.WithField("client", client.info.ID).Debug("Response sent to")
}

func (h *Hub) processCache(event *Event) {
	client, err := h.sessionStorage.getByID(event.SessionID)
	if err != nil {
		h.log.WithError(err).Warn("unable to get client by SessionID")
		h.rmSession(event.SessionID)
	}

	// send all broadcast events to the client
	broadcastEvents, err := h.cacheStorage.GetBroadcast()
	if err != nil {
		h.log.WithError(err).Warn("unable to get client by SessionID")
	}

	// send all direct event to client
	directEvents, err := h.cacheStorage.GetDirect(client.info.UserID)
	if err != nil {
		h.log.WithError(err).Warn("unable to get direct client cache")
	}

	for _, m := range append(broadcastEvents, directEvents...) {
		client.send <- &models.Message{
			Channel: m.Channel,
			Event:   m.Event,
			Command: m.Command,
			Data:    m.Data,
		}
	}
}
