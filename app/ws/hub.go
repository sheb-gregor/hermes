package ws

import (
	"context"
	"encoding/json"
	"runtime"
	"time"

	"hermes/cache"
	"hermes/config"
	"hermes/metrics"
	"hermes/models"

	"github.com/lancer-kit/uwe/v2"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Hub maintains the set of active sessionStorage and broadcasts MessagesChan to the connected clients.
type Hub struct {
	ctx    context.Context
	cancel context.CancelFunc
	log    zerolog.Logger

	cacheStorage cache.Storage

	eventStream    EventStream
	sessionStorage SessionStorage
}

func NewHub(logger zerolog.Logger, cfg config.CacheCfg) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	cacheStorage, err := cache.NewStorage(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize cache storage")
	}

	return &Hub{
		log:          logger.With().Str("sub_service", "ws-hub").Logger(),
		ctx:          ctx,
		cancel:       cancel,
		cacheStorage: cacheStorage,

		sessionStorage: newSessionStorage(),
		eventStream:    make(EventStream, 256),
	}
}

type HubCommunicator struct {
	EventBus    EventStream
	GetSessions func() map[string]map[int64]models.SessionInfo
	LogProvider func() zerolog.Logger
}

func (h *Hub) Communicator() HubCommunicator {
	return HubCommunicator{
		EventBus:    h.eventStream,
		GetSessions: h.sessionStorage.ListUsersSessions,
		LogProvider: func() zerolog.Logger { return h.log },
	}
}

func (h *Hub) EventBus() EventStream { return h.eventStream }

func (h *Hub) Context() context.Context { return h.ctx }

func (h *Hub) addSession(client *Session) {
	h.sessionStorage.AddSession(client.info.ID, client)

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writeToStream()
	go client.readStream()
	metrics.Inc(config.WSActiveSessions)
}

func (h *Hub) authorizeSession(session models.SessionInfo) {
	h.sessionStorage.AddUserSession(session.UserID, session)
	metrics.Inc(config.WSAuthorizedSessions)
}

func (h *Hub) rmSession(sessionID int64) {
	client := h.sessionStorage.RMSession(sessionID)
	if client == nil {
		h.log.Warn().Int64("connUID", sessionID).
			Msg("unable to get client by connUID")
		return
	}

	if err := client.Close(); err != nil {
		h.log.Error().Err(err).
			Int64("connUID", sessionID).
			Msg("failed to close client session")
	}

	metrics.Inc(config.WSClosedSessions)
	metrics.Dec(config.WSActiveSessions)
	metrics.Dec(config.WSAuthorizedSessions)
}

func (h Hub) GetSessionsCount() int64 {
	return h.sessionStorage.GetSessionsCount()
}

func (h *Hub) Init() error {
	return nil
}

func (h *Hub) Run(wCtx uwe.Context) error {
	gcTicker := time.NewTicker(2 * time.Hour)

	for {
		select {
		case <-gcTicker.C:
			h.log.Info().Msg("force garbage collection running...")
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
					h.log.Warn().Err(err).Msg("failed to process message")
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
	h.sessionStorage.RMSessions(func(id int64, session *Session) {
		if err := session.Close(); err != nil {
			h.log.Error().Err(err).Int64("sessionID", id).
				Msg("failed to close client session")
		}

		metrics.Inc(config.WSClosedSessions)
		metrics.Dec(config.WSActiveSessions)
		if session.info.UserID != "" {
			metrics.Dec(config.WSAuthorizedSessions)
		}
	})
}

func (h *Hub) broadCastAll(message *models.Message) {
	h.sessionStorage.ForEach(func(id int64, session *Session) {
		if message.Meta.Role != models.VRoleAny && session.info.Role != message.Meta.Role {
			return
		}

		session.send <- message
		metrics.Inc(config.SentBroadcastMessages)
	})
}

func (h *Hub) sendDirect(message *models.Message) {
	for sessionID := range h.sessionStorage.GetUserSessions(message.Meta.UserUID) {
		session := h.sessionStorage.GetSession(sessionID)
		if session == nil {
			h.log.Error().Int64("sessionID", sessionID).Msg("unable to get session by id")
			continue
		}

		if message.Meta.Role != models.VRoleAny && session.info.Role != message.Meta.Role {
			h.log.Error().Str("event_role", message.Meta.Role).
				Str("session_role", session.info.Role).
				Msg("user role mismatch with required by event")
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
	client := h.sessionStorage.GetSession(sessionID)
	if client == nil {
		h.log.Warn().Int64("sessionID", sessionID).Msg("unable to get client by SessionID")
		h.rmSession(sessionID)
		return
	}
	client.send <- &models.Message{Event: EvHandshake, Channel: EvStatusChannel}
	h.log.Debug().Int64("sessionID", sessionID).Msg("response sent")
}

func (h *Hub) processCache(event *Event) {
	client := h.sessionStorage.GetSession(event.SessionID)
	if client == nil {
		h.log.Warn().Int64("sessionID", event.SessionID).Msg("unable to get client by SessionID")
		h.rmSession(event.SessionID)
		return
	}

	// send all broadcast events to the client
	broadcastEvents, err := h.cacheStorage.GetBroadcast()
	if err != nil {
		h.log.Error().Err(err).Int64("sessionID", event.SessionID).
			Msg("unable to get broadcast events")
	}

	// send all direct event to client
	directEvents, err := h.cacheStorage.GetDirect(client.info.UserID)
	if err != nil {
		h.log.Error().Err(err).Int64("sessionID", event.SessionID).
			Msg("unable to get directs events")
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
