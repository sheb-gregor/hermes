package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/metrics"
	"gopkg.in/yaml.v2"
)

func main() {
	cfg := getConfig()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())

	// exchangeRate := (len(cfg.RabbitMQ.Subs) * cfg.ConnNumber.ConnPercentage) / 100

	wg := sync.WaitGroup{}
	mqEmitter := NewRabbitEmitter(cfg.RabbitMQ.Auth.URL(), cfg)

	wg.Add(1)
	go func() {
		mqEmitter.metrics.Collect(ctx)
		wg.Done()
	}()

	mqEmitter.metrics.PrettyPrint = false

	for i, mqSub := range cfg.RabbitMQ.Subs {
		wg.Add(1)
		go func(sub config.Exchange, i int) {
			defer wg.Done()

			emitterChannelsMKey := metrics.MKey("subs")
			mqPublisher, err := NewRabbitPublisher(mqEmitter.uri, cfg)
			if err != nil {
				log.Printf("failed to create mq submitter: %s", err)
				return
			}
			mqEmitter.metrics.Add(emitterChannelsMKey)
			timer := time.NewTimer(time.Minute * 5)
			send := time.NewTicker(time.Microsecond * 50)
			for {
				select {
				case <-timer.C:
					mqPublisher.Close()
					send.Stop()
					return

				case <-send.C:
					if i%2 == 0 {
						err = mqPublisher.publishMessage(sub, directExchange, mqEmitter.metrics)
						if err != nil {
							log.Printf("failed to publish message %s", err)
							return
						}
					} else {
						err = mqPublisher.publishMessage(sub, broadcastExchange, mqEmitter.metrics)
						if err != nil {
							log.Printf("failed to publish message %s", err)
							return
						}
					}
				}
			}
		}(mqSub, i)
	}
	<-interrupt
	cancel()
	log.Println("interrupt")

	mqEmitter.SaveMetrics()
}

func getConfig() RabbitEmitterCfg {
	cfgPath := flag.String("conf", "rabbit_emitter.yaml", "path to config")
	flag.Parse()

	var cfg RabbitEmitterCfg
	yamlFile, err := ioutil.ReadFile(*cfgPath)
	if err != nil {
		log.Fatalf("can`t read confg file: %s", err)
	}
	err = yaml.Unmarshal(yamlFile, &cfg)
	if err != nil {
		log.Fatalf("can`t unmarshal the config file: %s", err)
	}
	return cfg
}
