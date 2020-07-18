package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	mc "hermes/metrics"
	"hermes/models"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	ChannelWsStatus = "ws_status"
	EventHandshake  = "handshake"
	EventSubscribe  = "subscribe"
	EventAuthorize  = "authorize"
	EventCache      = "cache"
	EventPing       = "ping"
	EventPong       = "pong"
)

type ClientAuth struct {
	Token  string `json:"token"`
	Origin string `json:"origin"`
	Role   string `json:"role"`
}

func (cfg ClientAuth) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Token, validation.Required),
		validation.Field(&cfg.Origin, validation.Required),
		validation.Field(&cfg.Role, validation.Required),
	)
}

type Event struct {
	Message Message
	Error   error
	Status  int
}

const (
	StatusTerminate  = -2
	StatusError      = -1
	StatusOk         = 0
	StatusNormalStop = 3
)

type Message struct {
	Channel string                 `json:"channel,omitempty"`
	Event   string                 `json:"event,omitempty"`
	Command map[string]string      `json:"command,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

type HermesClient struct {
	ws         *websocket.Conn
	auth       *ClientAuth
	metricsAdd func(key mc.MKey)
}

func NewClient(ctx context.Context, hermesURL string, metricsAdd func(key mc.MKey)) (*HermesClient, error) {
	return NewClientWihDialer(ctx, hermesURL, metricsAdd, &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
	})
}

func NewClientWihDialer(ctx context.Context, hermesURL string, metricsAdd func(key mc.MKey), dialer *websocket.Dialer) (*HermesClient, error) {
	conn, _, err := dialer.DialContext(ctx, hermesURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start the ws connection")
	}

	return &HermesClient{ws: conn, metricsAdd: metricsAdd}, nil
}

func (c *HermesClient) Close() error {
	return c.ws.Close()
}

func (c *HermesClient) Handshake() error {
	msg := Message{
		Channel: ChannelWsStatus,
		Event:   EventHandshake,
	}
	return c.Send(msg)
}

func (c *HermesClient) Subscribe(channel, event string) error {
	msg := Message{
		Channel: ChannelWsStatus,
		Event:   EventSubscribe,
		Command: map[string]string{},
	}

	msg.Command["channel"] = channel
	msg.Command["event"] = event
	c.metricsAdd(mc.MKey("hermes-client" + mc.Separator + "subscribe" + mc.Separator + channel + mc.Separator + event))
	return c.Send(msg)
}

func (c *HermesClient) RequestCache(cacheFilter map[string]string) error {
	msg := Message{
		Channel: ChannelWsStatus,
		Event:   EventCache,
		Command: cacheFilter,
	}
	return c.Send(msg)
}

func (c *HermesClient) Authorize(cfg ClientAuth) error {
	c.auth = &cfg

	authMsg := map[string]string{
		models.FieldToken:  cfg.Token,
		models.FieldOrigin: cfg.Origin,
		models.FieldRole:   cfg.Role,
	}

	msg := Message{
		Channel: ChannelWsStatus,
		Event:   EventAuthorize,
		Command: authMsg,
	}

	c.metricsAdd(mc.MKey("hermes-client" + mc.Separator + "authorize" + mc.Separator + cfg.Token))
	return c.Send(msg)
}

func (c *HermesClient) Send(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.ws.WriteMessage(websocket.TextMessage, data)
}

func (c *HermesClient) Listen(ctx context.Context, messageBus chan<- Event) {

	done := make(chan struct{})
	incomingMessages := make(chan wsMessage, 2)
	readerCtx, cancelReader := context.WithCancel(ctx)
	go c.readWsMessages(readerCtx, incomingMessages, done)

	for {
		select {
		// receive done request
		case <-ctx.Done():
			cancelReader()
			<-done
			messageBus <- Event{Status: StatusNormalStop}
			return

		// read data from websocket connection
		case rawMsg := <-incomingMessages:
			if rawMsg.Error != nil {
				messageBus <- Event{
					Status: StatusTerminate,
					Error:  errors.Wrap(rawMsg.Error, "read error; disconnecting")}

				cancelReader()
				return
			}

			var msg Message
			err := json.Unmarshal(rawMsg.Body, &msg)
			if err != nil {
				messageBus <- Event{
					Status: StatusError,
					Error:  errors.Wrap(err, "can`t unmarshal the incoming message")}
				continue
			}

			switch msg.Event {
			case EventPing:
				pongMsg := Message{Channel: ChannelWsStatus, Event: EventPong}
				err = c.Send(pongMsg)
				c.metricsAdd(mc.MKey("hermes-client" + mc.Separator + "receive" + mc.Separator + "ping"))

			case EventAuthorize:
				if msg.Data["authorize"] != "success" {
					messageBus <- Event{
						Status:  StatusError,
						Message: msg}
				}

				c.metricsAdd(mc.MKey("hermes-client" + mc.Separator + "receive" + mc.Separator + "authorized"))
			default:
				messageBus <- Event{Status: StatusOk, Message: msg}
				c.metricsAdd(mc.MKey("hermes-client" + mc.Separator + "receive" + mc.Separator + "event"))
				c.metricsAdd(mc.MKey("hermes-client" + mc.Separator + "receive" +
					mc.Separator + "event" + mc.Separator + msg.Event))
			}
		}
	}
}

func (c *HermesClient) readWsMessages(ctx context.Context, messages chan<- wsMessage, done chan<- struct{}) {
	for {
		select {
		case <-ctx.Done():
			close(messages)
			done <- struct{}{}
			return

		default:
			msgCode, message, err := c.ws.ReadMessage()
			if err != nil {
				messages <- wsMessage{Error: err}
				continue
			}

			if message == nil {
				continue
			}
			messages <- wsMessage{Body: message, Type: msgCode}
		}
	}
}

type wsMessage struct {
	Type  int
	Body  []byte
	Error error
}
