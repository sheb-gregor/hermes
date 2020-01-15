package socket

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nats-io/go-nats"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.inn4science.com/ctp/hermes/models"
)

const (
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512

	// Key for wildcard subscriptions
	WildcardSubscription = "***"
)

const (
	EvHandshake     = "handshake"
	EvSubscribe     = "subscribe"
	EvUnsubscribe   = "unsubscribe"
	EvStatusChannel = "ws_status"
	EvPong          = "pong"
)

const (
	unableToUnmarshal = "unable to unmarshal json:"
	unableToMarshal   = "unable to marshal data:"
)

// Session is a middleman between the websocket connection and the hub.
type Session struct {
	conn *websocket.Conn

	connUID  int64
	userUUID string

	bus    EventStream
	ctx    context.Context
	cancel context.CancelFunc

	// Buffered channel of outbound MessagesChan.
	send                 chan *models.Message
	log                  *logrus.Entry
	subscriptionsChannel activeChannel
	subscriptionsEvent   activeEvent
}

func NewSession(ctx context.Context, log *logrus.Entry, bus EventStream,
	conn *websocket.Conn, uid int64, userUUID string) *Session {

	ctx, cancel := context.WithCancel(ctx)

	return &Session{
		ctx:                  ctx,
		cancel:               cancel,
		conn:                 conn,
		bus:                  bus,
		connUID:              uid,
		userUUID:             userUUID,
		log:                  log.WithField("connUID", uid).WithField("userUUID", userUUID),
		send:                 make(chan *models.Message, nats.DefaultMaxChanLen*2),
		subscriptionsChannel: activeChannel{new(sync.Map)},
		subscriptionsEvent:   activeEvent{new(sync.Map)},
	}
}

func (c *Session) isSubscribed(channel, event string) bool {
	chanSub := c.subscriptionsChannel.getChannel(channel) || c.subscriptionsChannel.getChannel(WildcardSubscription)

	eventSubs := c.subscriptionsEvent.getSubscribeMap(channel)
	if eventSubs == nil {
		return false
	}

	eventSub := eventSubs[event] || eventSubs[WildcardSubscription]
	return chanSub && eventSub
}

func (c *Session) addSubscription(channel, event string) {
	// MetricsCollector.Add(metrics.MKey("sessionStorage." + c.userUUID + ".addSubscription"))
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

// readStream pumps MessagesChan from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Session) readStream() {
	defer func() {
		c.log.Info("connection closed for user:", c.connUID)
		c.bus <- &Event{Kind: EKUnregister, SessionID: c.connUID}
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPongHandler(func(string) error { return c.conn.SetReadDeadline(time.Now().Add(pongWait)) })

	var needToStop bool
	incomingMessages := make(chan []byte)

	go func(im chan []byte) {
		for {
			if needToStop {
				close(im)
				return
			}
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.log.WithError(err).Info("socket closed")
				} else if err != io.EOF {
					c.log.Info("error while reading from client:", err)
				}
				return
			}
			// MetricsCollector.Add(metrics.MKey("sessionStorage." + c.userUUID + ".readMessage"))
			if message == nil {
				c.log.Debug("nil message from read channel")
				continue
			}

			im <- message
		}
	}(incomingMessages)

	for {
		select {
		case <-c.ctx.Done():
			// all the necessary things will be done on a deferred call
			needToStop = true
			return
		case message := <-incomingMessages:
			if err := c.processIncomingMessage(message); err != nil {
				c.log.WithError(err).Info("failed to check message event")
				continue
			}
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
		c.log.Info("close connection for user:", c.connUID)

		c.bus <- &Event{Kind: EKUnregister, SessionID: c.connUID}
	}()

	for {
		select {
		case <-c.ctx.Done():
			// all the necessary things will be done on a deferred call
			return
		case message, ok := <-c.send:
			if !ok {
				c.log.
					Debug("ending client write handler")
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
				c.log.Info("error when writing to client: ", err)
				return
			}

		case <-ticker.C:
			if err := c.pingWs(); err != nil {
				c.log.WithError(err).Info("failed to ping socket")
				return
			}
		}
	}
}

func (c *Session) pingWs() error {
	rawData := []byte(`{"channel":"ws_status","event":"ping"}`)
	// rawData, err := json.Marshal(models.Message{Channel: EvStatusChannel, Event: "ping"})
	// if err != nil {
	// 	return errors.Wrap(err, "unable to marshal ping message")
	// }

	if err := c.conn.WriteMessage(websocket.TextMessage, rawData); err != nil {
		c.log.WithError(err).Info("failed to send ping message")
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

	switch userMsg.Event {
	case EvHandshake:
		// MetricsCollector.Add(metrics.MKey("sessionStorage." + c.userUUID + ".EvHandshake"))

		c.bus <- &Event{Kind: EKHandshake, SessionID: c.connUID}
	case EvSubscribe:
		// MetricsCollector.Add(metrics.MKey("sessionStorage." + c.userUUID + ".EvSubscribe"))

		channel := userMsg.Command["channel"]
		event := userMsg.Command["event"]
		c.addSubscription(channel, event)

	case EvUnsubscribe:
		// MetricsCollector.Add(metrics.MKey("sessionStorage." + c.userUUID + ".EvUnsubscribe"))

		channel := userMsg.Command["channel"]
		c.rmSubscription(channel)
	case EvPong:
		if userMsg.Channel != EvStatusChannel {
			return errors.New("invalid ping status channel")
		}

		c.log.Debug("client synchronization - pong received")
	}

	return nil
}

func (c *Session) writeToClient(message *models.Message) error {
	// MetricsCollector.Add(metrics.MKey("sessionStorage." + c.userUUID + ".writeToClient"))

	data, err := json.Marshal(message)
	if err != nil {
		c.log.WithError(err).
			WithField("handler", "writeToClient").
			Error(unableToMarshal, message)
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}
