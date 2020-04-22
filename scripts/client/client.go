package main

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/metrics"
	"gitlab.inn4science.com/ctp/hermes/models"
)

const (
	channelBufSize   = 1000
	connectPeriod    = 10 * time.Millisecond
	authPeriod       = connectPeriod * 5
	disconnectPeriod = connectPeriod * 3

	evWsStatus  = "ws_status"
	evHandshake = "handshake"
	evSubscribe = "subscribe"
	evAuthorize = "authorize"
	evCache     = "cache"
	evPing      = "ping"
	evPong      = "pong"
)

type WsConnector struct {
	wsClientsStorage map[string]*WsClient
	cfg              ClientCfg
	ctx              context.Context

	metrics *metrics.SafeMetrics
}

func NewWsConnector(cfg ClientCfg, ctx context.Context) *WsConnector {
	clients := make(map[string]*WsClient)

	return &WsConnector{
		wsClientsStorage: clients,
		cfg:              cfg,
		ctx:              ctx,
		metrics:          new(metrics.SafeMetrics).New(ctx),
	}
}

// schedules the subscribe/unsubscribe ws sockets
func (c *WsConnector) RunWsScheduler() {
	connect := time.NewTicker(connectPeriod)
	auth := time.NewTicker(authPeriod)
	disconnect := time.NewTicker(disconnectPeriod)

	rand.Seed(time.Now().UnixNano())
	ctx, cancel := context.WithDeadline(c.ctx, time.Now().Add(time.Minute*5))

	// cacheFilter consists of
	// - channel name (name/*** - wildcard)
	// - event (optional)
	// cacheFilter := map[string]string{
	// 	"ctp.notifications.prices": "",
	// 	"ctp.hermes.deposits":      "",
	// }
	for {
		select {
		case <-ctx.Done():
			for _, client := range c.wsClientsStorage {
				client.closeWsConnection()
			}
			cancel()

			connect.Stop()
			auth.Stop()
			disconnect.Stop()
			return

		case <-connect.C:
			client := c.addWsClient(ctx)

			// - handshake
			// - subscribe to all config channels
			// - get cache
			client.handshake()
			for _, ch := range c.cfg.RabbitMQ.Subs {
				client.subscribeToChannel(ch)
				// client.getCache(cacheFilter)
			}

		case <-auth.C:
			client := c.addWsClient(ctx)
			client.auth(c.cfg.Auth)
			// client.getCache(cacheFilter)

		case <-disconnect.C:
			c.metrics.Add("disconnected")

			log.Println("\ndisconnect scheduler")

			// randomly close websocket connection
			key := randMapKey(c.wsClientsStorage)
			log.Printf("user to disconnect %s", key)
			client := c.wsClientsStorage[key]

			if client != nil {
				client.ctx.Done()
				client.closeWsConnection()
				delete(c.wsClientsStorage, key)
			}
		}
	}
}

func (c *WsConnector) SaveMetrics() {
	data, err := c.metrics.MarshalJSON()
	if err != nil {
		log.Fatalf("failed to marshal the metrics: %s", err)
	}
	err = ioutil.WriteFile("client_metrics_report.json", data, 0644)
	if err != nil {
		log.Printf("failed to write the metrics: %s", err)
	}
}

func randMapKey(m map[string]*WsClient) string {
	if len(m) == 0 {
		return ""
	}
	mapKeys := make([]string, 0, len(m)) // pre-allocate exact size
	for key := range m {
		mapKeys = append(mapKeys, key)
	}
	return mapKeys[rand.Intn(len(mapKeys))]
}

type WsClient struct {
	id  string
	ctx context.Context
	ws  *websocket.Conn
	ch  chan *models.Message
}

type Message struct {
	Channel string            `json:"channel"`
	Event   string            `json:"event"`
	Command map[string]string `json:"command"`
}

func NewWsClient(hermesURL string, ctx context.Context) *WsClient {
	id := uuid.New().String()
	conn, err := dial(hermesURL)
	if err != nil {
		log.Fatalf("dial %s", err)
	}

	return &WsClient{
		id:  id,
		ctx: ctx,
		ws:  conn,
		ch:  make(chan *models.Message, channelBufSize),
	}
}

func (c *WsConnector) addWsClient(ctx context.Context) *WsClient {
	c.metrics.Add("connected")
	log.Println("\nconnect scheduler")

	client := NewWsClient(c.cfg.HermesURL, ctx)
	// add new websocket client to ws storage
	c.wsClientsStorage[client.id] = client
	go client.listenRead(c.metrics)
	return client
}

func dial(url string) (*websocket.Conn, error) {
	log.Printf("connecting to %s", url)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start the ws connection")
	}
	return conn, err
}

func (c *WsClient) Listen(metric *metrics.SafeMetrics) {
	go c.listenRead(metric) // receive new messages from ws server
}

func (c *WsClient) listenRead(metric *metrics.SafeMetrics) {
	log.Printf("listening read from client %s", c.id)
	readerCtx, cancelReader := context.WithCancel(c.ctx)

	incomingMessages := make(chan wsMessage, 2)
	go c.readWsMessages(readerCtx, incomingMessages)

	for {
		select {
		// receive done request
		case <-c.ctx.Done():
			close(c.ch)
			cancelReader()
			return

		// read data from websocket connection
		case rawMsg := <-incomingMessages:
			// log.Printf("incoming message %s", string(rawMsg.data))
			var msg models.Message
			err := json.Unmarshal(rawMsg.data, &msg)
			if err != nil {
				log.Fatalf("can`t unmarshal the incoming message %s", err)
			}

			// Collect metrics
			// - number of received events
			// - number of events by type
			// - user list by event type
			// - received events-by-type by some user
			metric.Add("receivedEvents")
			metric.Add(metrics.MKey("overallEvents." + msg.Event))
			metric.Add(metrics.MKey("usersByEventType." + msg.Event + "." + c.id))
			metric.Add(metrics.MKey("userEventDetails." + c.id + ".eventType." + msg.Event))

			switch msg.Event {
			case evPing:
				pongMsg := Message{
					Channel: evWsStatus,
					Event:   evPong,
				}
				c.writeMessageToWs(pongMsg)
				log.Printf("write pong for client %s", c.id)

			case evAuthorize:
				if msg.Data["authorize"] == "success" {
					metric.Add(metrics.MKey("authorized"))
				}

			default:
				log.Printf("received msg %v from client %s", msg, c.id)
			}
		}
	}
}

type wsMessage struct {
	data  []byte
	mType int
}

func (c *WsClient) readWsMessages(ctx context.Context, incomingMessages chan wsMessage) {
	for {
		log.Printf("read ws message %s", c.id)
		select {
		case <-ctx.Done():
			log.Println("done ctx read ws message")
			close(incomingMessages)
			return

		default:
			log.Println("listen to ws message")
			msgCode, message, err := c.ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Println("socket closed")
				} else if err != io.EOF {
					log.Println("error while reading from client")
				}
				return
			}

			if message == nil {
				log.Println("nil message from read channel")
				continue
			}
			incomingMessages <- wsMessage{data: message, mType: msgCode}
		}
	}
}

func (c *WsClient) writeMessageToWs(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Fatalf("failed marshal the mq message: %s", err)
	}

	err = c.ws.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Println("write to websocket err:", err)
		return
	}
}

func (c *WsClient) handshake() {
	msg := Message{
		Channel: evWsStatus,
		Event:   evHandshake,
	}
	c.writeMessageToWs(msg)
	log.Printf("sent ws handshake for client %s", c.id)
}

func (c *WsClient) getCache(cacheFilter map[string]string) {
	msg := Message{
		Channel: evWsStatus,
		Event:   evCache,
		Command: cacheFilter,
	}
	c.writeMessageToWs(msg)
	log.Printf("sent ws cache request for client %s", c.id)
}

func (c *WsClient) auth(cfg ClientAuth) {
	authMsg := map[string]string{
		models.FieldToken:  cfg.Token,
		models.FieldOrigin: cfg.Origin,
		models.FieldRole:   cfg.Role,
	}
	msg := Message{
		Channel: evWsStatus,
		Event:   evAuthorize,
		Command: authMsg,
	}

	c.writeMessageToWs(msg)
	log.Print("sent auth connection")
}

func (c *WsClient) subscribeToChannel(cfg config.Exchange) {
	msg := Message{
		Channel: evWsStatus,
		Event:   evSubscribe,
		Command: map[string]string{},
	}
	msg.Command["channel"] = cfg.Queue
	c.writeMessageToWs(msg)
	log.Printf("client %s subscribed to %s", c.id, cfg.Queue)
}

func (c *WsClient) closeWsConnection() {
	err := c.ws.Close()
	if err != nil {
		log.Printf("close websocket connection for client %s err %s", c.id, err)
	}
}
