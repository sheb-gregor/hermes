package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"gitlab.inn4science.com/ctp/hermes/metrics"

	"github.com/gorilla/websocket"
)

var metricsCollector *metrics.SafeMetrics

func main() {
	log.SetFlags(0)

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	metricsCollector = &metrics.SafeMetrics{}
	metricsCollector = metricsCollector.New(ctx)
	wg.Add(1)
	go func() {
		metricsCollector.Collect()
		wg.Done()
	}()
	metricsCollector.PrettyPrint = true
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		time.Sleep(15 * time.Millisecond)
		name := fmt.Sprintf("%d", i)
		go func(name string) {
			sub(ctx, name)
			wg.Done()
		}(name)
	}
	time.Sleep(30 * time.Second)
	//<-interrupt
	cancel()

	log.Println("interrupt")

	wg.Wait()
	data, _ := metricsCollector.MarshalJSON()
	_ = ioutil.WriteFile("metrics_report."+time.Now().String()+".json", data, 0644)

}
func sub(ctx context.Context, name string) {

	u := url.URL{Scheme: "ws", Host: "5.196.93.21:9000", Path: "/_ws/subscribe"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			//log.Printf("recv: %s", message)
			metricsCollector.Add(metrics.MKey("client." + name + ".in"))

			if strings.Contains(string(message), "ping") {
				d := map[string]string{
					"event":   "pong",
					"channel": "ws_status",
				}
				data, _ := json.Marshal(d)
				err = c.WriteMessage(websocket.TextMessage, data)
				log.Println("write pong:", err)
				metricsCollector.Add(metrics.MKey("client." + name + ".out"))
			}
		}
	}()

	//ticker := time.NewTicker(time.Second)
	//defer ticker.Stop()
	channelList := []string{
		"rates",
		"rates_by_exchangers",
		"rates_global",
		"rates_global_statistics",
		"market_live_order_book_ask",
		"market_live_order_book_bids",
		"market_trades",
		"blockchain_statistics",
		"pool_txs"}
	subData := struct {
		Event   string            `json:"event"`
		Command map[string]string `json:"command"`
	}{
		Event:   "subscribe",
		Command: map[string]string{},
	}

	for _, ch := range channelList {
		subData.Command["channel"] = ch
		data, _ := json.Marshal(subData)

		err = c.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			log.Println("write:", err)
			return
		}
		metricsCollector.Add(metrics.MKey("client." + name + ".out"))
	}

	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			log.Println("interrupt")
			return
		}
	}
}
