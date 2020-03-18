package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"gopkg.in/yaml.v2"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/metrics"
)

type client struct {
	cfg  ClientCfg
	name string
	ctx  context.Context
	ws   *websocket.Conn
}

type subData struct {
	Event   string            `json:"event"`
	Command map[string]string `json:"command"`
}

func NewClient(name, uuid string, cfg ClientCfg, ctx context.Context) *client {
	url := fmt.Sprintf("ws://%s:%d/_ws/subscribe?uuid=%v",
		cfg.API.Host,
		cfg.API.Port,
		uuid,
	)
	log.Printf("user uuid %s", uuid)
	c, err := dial(url)
	if err != nil {
		log.Fatalf("dial %s", err)
	}

	return &client{
		cfg:  cfg,
		name: name,
		ctx:  ctx,
		ws:   c,
	}
}

func getConfig(path string) ClientCfg {
	var cfg ClientCfg

	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("can`t read confg file: %s", err)
	}
	err = yaml.Unmarshal(yamlFile, &cfg)
	if err != nil {
		log.Fatalf("can`t unmarshal the config file: %s", err)
	}
	return cfg
}

func main() {
	cfg := getConfig("client.config.yaml")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())

	wg := sync.WaitGroup{}

	metricsCollector := new(metrics.SafeMetrics).New(ctx)
	wg.Add(1)
	go func() {
		metricsCollector.Collect()
		wg.Done()
	}()
	metricsCollector.PrettyPrint = true
	for i := 0; i < 10; i++ {
		wg.Add(1)
		time.Sleep(15 * time.Millisecond)
		name := fmt.Sprintf("%d", i)
		go func(name string) {
			uuid := uuid.New().String()
			client := NewClient(name, uuid, cfg, ctx)
			client.sub(cfg.RabbitMQ, metricsCollector)
			wg.Done()
		}(name)
	}

	<-interrupt
	cancel()

	log.Println("interrupt")

	wg.Wait()
	data, err := metricsCollector.MarshalJSON()
	if err != nil {
		log.Fatalf("failed to marshal the metrics: %s", err)
	}
	err = ioutil.WriteFile("metrics_report.json", data, 0644)
	if err != nil {
		log.Printf("failed to write the metrics: %s", err)
	}
}

func dial(url string) (*websocket.Conn, error) {
	log.Printf("connecting to %s", url)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start the ws connection")
	}
	return conn, err
}

func (client *client) sub(cfg config.RabbitMQ, metricsCollector *metrics.SafeMetrics) {
	clientInMKey := metrics.MKey("client." + client.name + ".in")
	clientOutMKey := metrics.MKey("client." + client.name + ".out")
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := client.ws.ReadMessage()
			if err != nil {
				log.Println("read error:", err)
				return
			}
			metricsCollector.Add(clientInMKey)

			if strings.Contains(string(message), "ping") {
				d := map[string]string{
					"event":   "pong",
					"channel": "ws_status",
				}
				data, err := json.Marshal(d)
				if err != nil {
					log.Fatalf("failed marshal the message: %s", err)
				}
				err = client.ws.WriteMessage(websocket.TextMessage, data)
				if err != nil {
					log.Printf("failed write the message: %s", err)
				}
				log.Println("write pong:", err)
				metricsCollector.Add(clientOutMKey)
			}
		}
	}()

	for _, ch := range cfg.Subs {
		client.writeMsg(ch)
		metricsCollector.Add(clientOutMKey)
	}

	for {
		select {
		case <-done:
			return
		case <-client.ctx.Done():
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := client.ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}

func (client *client) writeMsg(subsConfig config.MqSubscription) {
	subs := subData{
		Event:   "subscribe",
		Command: map[string]string{},
	}

	subs.Command["channel"] = subsConfig.Queue
	data, err := json.Marshal(subs)
	if err != nil {
		log.Fatalf("failed marshal the sub data: %s", err)
	}

	err = client.ws.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Println("write to websocket err:", err)
		return
	}
}
