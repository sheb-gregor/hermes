package ws

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.inn4science.com/ctp/hermes/models"
)

const (
	// Time allowed to read the next pong message from the peer.
	pongWait = 30 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512

	maxChanLen = 256

	// Key for wildcard subscriptions
	WildcardSubscription = "***"
)

const (
	EvHandshake     = "handshake"
	EvAuthorize     = "authorize"
	EvSubscribe     = "subscribe"
	EvUnsubscribe   = "unsubscribe"
	EvStatusChannel = "ws_status"
	EvPong          = "pong"
	EvCache         = "cache"
)

const (
	unableToUnmarshal = "unable to unmarshal json:"
	unableToMarshal   = "unable to marshal data:"
)

type AuthProviderF func(models.AuthRequest) (*models.AuthResponse, int, error)

// Session is a middleman between the websocket connection and the hub.
type Session struct {
	conn *websocket.Conn

	bus    EventStream
	ctx    context.Context
	cancel context.CancelFunc

	info models.SessionInfo

	// Buffered channel of outbound MessagesChan.
	send                 chan *models.Message
	log                  *logrus.Entry
	subscriptionsChannel activeChannel
	subscriptionsEvent   activeEvent

	authProvider AuthProviderF
}

func NewSession(ctx context.Context, log *logrus.Entry, bus EventStream,
	conn *websocket.Conn, info models.SessionInfo, authProvider AuthProviderF) *Session {

	ctx, cancel := context.WithCancel(ctx)

	return &Session{
		ctx:                  ctx,
		cancel:               cancel,
		conn:                 conn,
		bus:                  bus,
		info:                 info,
		authProvider:         authProvider,
		log:                  log.WithField("session_id", info.ID),
		send:                 make(chan *models.Message, maxChanLen/2),
		subscriptionsChannel: activeChannel{new(sync.Map)},
		subscriptionsEvent:   activeEvent{new(sync.Map)},
	}
}

func (c *Session) isSubscribed(channel, event string) bool {
	if channel == EvCache || channel == EvStatusChannel {
		return true
	}

	if c.subscriptionsChannel.getChannel(WildcardSubscription) {
		eventSubs := c.subscriptionsEvent.getSubscribeMap(WildcardSubscription)
		if eventSubs == nil {
			return true
		}

		return eventSubs[event] || eventSubs[WildcardSubscription]
	}

	chanSub := c.subscriptionsChannel.getChannel(channel)
	eventSubs := c.subscriptionsEvent.getSubscribeMap(channel)
	if eventSubs == nil {
		return false
	}

	eventSub := eventSubs[event] || eventSubs[WildcardSubscription]
	return chanSub && eventSub
}

func (c *Session) addSubscription(channel, event string) {
	// MetricsCollector.Add(metrics.MKey("sessionStorage." + c.userUID + ".addSubscription"))
	if channel == "" {
		return
	}

	if event == "" {
		event = WildcardSubscription
	}

	c.subscriptionsChannel.Store(channel, true)

	eventSubs := c.subscriptionsEvent.getSubscribeMap(channel)
	if eventSubs == nil {
		eventSubs = make(map[string]bool)
	}

	eventSubs[event] = true
	c.subscriptionsEvent.Store(channel, eventSubs)
}

func (c *Session) rmSubscription(channel string) {
	c.subscriptionsChannel.Store(channel, false)
	c.subscriptionsEvent.Store(channel, map[string]bool{})
}

type wsMessage struct {
	data  []byte
	mType int
}

// readStream pumps MessagesChan from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
// nolint:funlen
func (c *Session) readStream() {
	defer func() {
		c.log.Info("connection closed")
		c.bus <- &Event{Kind: EKUnregister, SessionID: c.info.ID}
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPongHandler(func(string) error { return c.conn.SetReadDeadline(time.Now().Add(pongWait)) })

	readerCtx, cancelReader := context.WithCancel(c.ctx)
	incomingMessages := make(chan wsMessage, 2)

	go c.readMessages(readerCtx, incomingMessages)

	for {
		select {
		case <-c.ctx.Done():
			// all the necessary things will be done on a deferred call
			cancelReader()

			return
		case m := <-incomingMessages:
			switch m.mType {
			case websocket.PingMessage, websocket.PongMessage:
				c.log.Trace("ws proto synchronization")
				continue
			case websocket.CloseMessage:
				cancelReader()
				return
			}
			if err := c.processIncomingMessage(m.data); err != nil {
				c.log.WithError(err).Info("failed to check message event")
				continue
			}
		}
	}

}

func (c *Session) readMessages(ctx context.Context, im chan wsMessage) {
	for {
		select {
		case <-ctx.Done():
			close(im)
			return
		default:
			msgCode, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.log.WithError(err).Debug("socket closed")
				} else if err != io.EOF {
					c.log.Debug("error while reading from client:", err)
				}
				return
			}

			// MetricsCollector.Add(metrics.MKey("sessionStorage." + c.userUID + ".readMessage"))
			if message == nil {
				c.log.Debug("nil message from read channel")
				continue
			}

			im <- wsMessage{data: message, mType: msgCode}
		}
	}
}

// writeToStream pumps MessagesChan from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Session) writeToStream() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.log.Debug("close connection")

		c.bus <- &Event{Kind: EKUnregister, SessionID: c.info.ID}
	}()

	for {
		select {
		case <-c.ctx.Done():
			// all the necessary things will be done on a deferred call
			return
		case message, ok := <-c.send:
			if !ok {
				c.log.Debug("ending client write handler")
				return
			}

			if message == nil {
				c.log.Debug("nil message from read channel")
				continue
			}

			if !c.isSubscribed(message.Channel, message.Event) {
				continue
			}
			if err := c.writeToClient(message); err != nil {
				c.log.WithError(err).Debug("error when writing to client")
				return
			}

			c.log.WithField("event", message.Event).Trace("write message to connection")

		case <-ticker.C:
			if err := c.pingWs(); err != nil {
				c.log.WithError(err).Debug("failed to ping socket")
				return
			}
		}
	}
}

func (c *Session) pingWs() error {
	rawData := []byte(`{"channel":"ws_status","event":"ping"}`)
	if err := c.conn.WriteMessage(websocket.TextMessage, rawData); err != nil {
		c.log.WithError(err).Debug("failed to send ping message")
		return err
	}

	c.log.Debug("client synchronization - ping sent")
	return nil
}

func (c *Session) processIncomingMessage(raw []byte) error {
	userMsg := new(models.Message)
	err := json.Unmarshal(raw, userMsg)
	if err != nil {
		c.log.WithError(err).
			WithField("handler", "processIncomingMessage").
			Error(unableToUnmarshal, string(raw))
		return err
	}

	if userMsg.Channel != EvStatusChannel {
		return errors.New("invalid channel")
	}

	switch userMsg.Event {
	case EvHandshake:
		// MetricsCollector.Add(metrics.MKey("sessionStorage." + c.userUID + ".EvHandshake"))

		c.bus <- &Event{Kind: EKHandshake, SessionID: c.info.ID}

	case EvAuthorize:
		c.processAuthEvent(userMsg)

	case EvCache:
		c.bus <- &Event{Kind: EKCache, SessionID: c.info.ID, Message: userMsg}

	case EvSubscribe:
		// MetricsCollector.Add(metrics.MKey("sessionStorage." + c.userUID + ".EvSubscribe"))

		channel := userMsg.Command["channel"]
		event := userMsg.Command["event"]
		c.addSubscription(channel, event)

	case EvUnsubscribe:
		// MetricsCollector.Add(metrics.MKey("sessionStorage." + c.userUID + ".EvUnsubscribe"))

		channel := userMsg.Command["channel"]
		c.rmSubscription(channel)

	case EvPong:
		c.log.Debug("client synchronization - pong received")
	}

	return nil
}

func (c *Session) processAuthEvent(userMsg *models.Message) {
	resultStatus := "success"

	if c.info.UserID != "" {
		c.send <- &models.Message{Channel: EvStatusChannel, Event: EvAuthorize,
			Data: map[string]interface{}{EvAuthorize: resultStatus, "resultCode": http.StatusOK},
		}
		return
	}

	code, err := c.verifyAuth(userMsg.Command)
	if err != nil {
		resultStatus = "failed"
		c.log.WithError(err).Error("failed to verifyAuth")
	} else {
		c.bus <- &Event{Kind: EKAuthorize, SessionID: c.info.ID, SessionInfo: &c.info}
	}

	c.send <- &models.Message{Channel: EvStatusChannel, Event: EvAuthorize,
		Data: map[string]interface{}{EvAuthorize: resultStatus, "resultCode": code},
	}
}

func (c *Session) verifyAuth(command map[string]string) (int, error) {
	c.info.Token = command[models.FieldToken]
	c.info.Origin = command[models.FieldOrigin]
	c.info.Role = command[models.FieldRole]

	authData, code, err := c.authProvider(models.AuthRequest{
		SessionID: c.info.ID,
		Token:     c.info.Token,
		UserAgent: c.info.UserAgent,
		IP:        c.info.IP,
		Origin:    c.info.Origin,
		Role:      c.info.Role,
	})
	if err != nil {
		return code, err
	}

	c.info.UserID = authData.UserID
	// c.log = c.log.WithField("user_id", authData.UserID)
	return code, nil
}

func (c *Session) writeToClient(message *models.Message) error {
	// MetricsCollector.Add(metrics.MKey("sessionStorage." + c.userUID + ".writeToClient"))

	var msg interface{} = message
	if message.Channel != EvStatusChannel {
		msg = message.ToShort()
	}

	data, err := json.Marshal(msg)
	if err != nil {
		c.log.WithError(err).
			WithField("handler", "writeToClient").
			Error(unableToMarshal, message)
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}
