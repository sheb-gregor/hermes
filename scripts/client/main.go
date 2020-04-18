package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sync"

	"gopkg.in/yaml.v2"
)

func main() {
	cfg := getConfig("client.config.yaml")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())

	wg := sync.WaitGroup{}

	wsConnector := NewWsConnector(cfg, ctx)
	wsConnector.LoadMetrics()

	wg.Add(1)
	go func() {
		wsConnector.metrics.Collect()
		wg.Done()
	}()
	wsConnector.metrics.PrettyPrint = false

	go wsConnector.RunWsScheduler()

	<-interrupt
	cancel()
	log.Println("interrupt")

	wg.Wait()

	wsConnector.SaveMetrics()
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
