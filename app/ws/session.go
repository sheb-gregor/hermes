package ws

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"hermes/config"
	"hermes/metrics"
	"hermes/models"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

const (
	// Time allowed to read the next pong message from the peer.
	pongWait = 15 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	// pingPeriod = (pongWait * 9) / 10
	pingPeriod = 10 * time.Second

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
	EvMute          = "mute"
	EvStatusChannel = "ws_status"
	EvPong          = "pong"
	EvCache         = "cache"
)

const (
	unableToUnmarshal = "unable to unmarshal json"
	unableToMarshal   = "unable to marshal data:"
)

type AuthProviderFunc func(models.AuthRequest) (*models.AuthResponse, int, error)

// Session is a middleman between the websocket connection and the hub.
type Session struct {
	conn *websocket.Conn

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	log    func() *zerolog.Logger

	// Buffered channel of outbound MessagesChan.
	bus  EventStream
	send chan *models.Message

	info         models.SessionInfo
	subInfo      SubscriptionsStore
	authProvider AuthProviderFunc
}

func NewSession(pCtx context.Context, logProvider func() zerolog.Logger,
	bus EventStream, conn *websocket.Conn, info models.SessionInfo, authProvider AuthProviderFunc) *Session {

	ctx, cancel := context.WithCancel(pCtx)
	return &Session{
		ctx:          ctx,
		cancel:       cancel,
		conn:         conn,
		bus:          bus,
		info:         info,
		authProvider: authProvider,
		log: func() *zerolog.Logger {
			l := logProvider().With().Int64("session_id", info.ID).Logger()
			return &l
		},
		send:    make(chan *models.Message, maxChanLen/2),
		subInfo: NewSubscriptions(),
	}
}

func (c *Session) Close() error {
	c.cancel()
	c.wg.Wait()

	err := c.conn.Close()
	if err != nil {
		return err
	}

	close(c.send)
	return nil
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
		c.log().Info().Msg("connection closed")
		c.bus <- &Event{Kind: EKUnregister, SessionID: c.info.ID}
		c.wg.Done()
	}()

	c.wg.Add(1)
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
				c.log().Trace().Msg("ws proto synchronization")
				continue
			case websocket.CloseMessage:
				cancelReader()
				return
			}
			if err := c.processIncomingMessage(m.data); err != nil {
				c.log().Warn().Err(err).Msg("failed to check message event")
				continue
			}
		}
	}

}

func (c *Session) readMessages(ctx context.Context, im chan wsMessage) {
	c.wg.Add(1)
	for {
		select {
		case <-ctx.Done():
			close(im)
			c.wg.Done()
			return
		default:
			msgCode, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.log().Warn().Err(err).Msg("socket closed")
				} else if err != io.EOF {
					c.log().Warn().Err(err).Msg("error while reading from client")
				}
				return
			}

			if message == nil {
				c.log().Warn().Msg("nil message from read channel")
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
		c.log().Debug().Msg("close connection")

		c.bus <- &Event{Kind: EKUnregister, SessionID: c.info.ID}
		c.wg.Done()
	}()
	c.wg.Add(1)

	for {
		select {
		case <-c.ctx.Done():
			// all the necessary things will be done on a deferred call
			return
		case message, ok := <-c.send:
			if !ok {
				c.log().Debug().Msg("ending client write handler")
				return
			}

			if message == nil {
				c.log().Debug().Msg("nil message from read channel")
				continue
			}

			if !c.subInfo.IsSubscribed(message.Channel, message.Event) {
				metrics.Inc(config.DroppedMessages)
				continue
			}
			if err := c.writeToClient(message); err != nil {
				c.log().Warn().Err(err).Msg("error when writing to client")
				return
			}

			metrics.Inc(config.DeliveredMessages)

		case <-ticker.C:
			if err := c.pingWs(); err != nil {
				c.log().Warn().Err(err).Msg("failed to ping socket")
				return
			}
		}
	}
}

func (c *Session) pingWs() error {
	rawData := []byte(`{"channel":"ws_status","event":"ping"}`)
	if err := c.conn.WriteMessage(websocket.TextMessage, rawData); err != nil {
		return err
	}

	c.log().Debug().Msg("client synchronization - ping sent")
	return nil
}

func (c *Session) processIncomingMessage(raw []byte) error {
	userMsg := new(models.Message)
	err := json.Unmarshal(raw, userMsg)
	if err != nil {
		c.log().Error().Err(err).
			Str("handler", "processIncomingMessage").
			Msg(unableToUnmarshal)
		return err
	}

	if userMsg.Channel != EvStatusChannel {
		return errors.New("invalid channel")
	}

	switch userMsg.Event {
	case EvHandshake:
		c.bus <- &Event{Kind: EKHandshake, SessionID: c.info.ID}

	case EvAuthorize:
		c.processAuthEvent(userMsg)

	case EvCache:
		c.bus <- &Event{Kind: EKCache, SessionID: c.info.ID, Message: userMsg}

	case EvSubscribe:
		channel := userMsg.Command["channel"]
		event := userMsg.Command["event"]
		c.subInfo.AddSubscription(channel, event)

	case EvUnsubscribe:
		channel := userMsg.Command["channel"]
		c.subInfo.RmSubscription(channel)

	case EvMute:
		event := userMsg.Command["event"]
		c.subInfo.MuteEvent(event)

	case EvPong:
		c.log().Debug().Msg("client synchronization - pong received")
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
		c.log().Error().Err(err).Msg("failed to verifyAuth")
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
	return code, nil
}

func (c *Session) writeToClient(message *models.Message) error {
	var msg interface{} = message
	if message.Channel != EvStatusChannel {
		msg = message.ToShort()
	}

	data, err := json.Marshal(msg)
	if err != nil {
		c.log().Error().Err(err).Msg(unableToMarshal)
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}
