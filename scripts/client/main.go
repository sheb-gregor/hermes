package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sync"

	"gopkg.in/yaml.v2"
)

func main() {
	cfgPath := flag.String("conf", "client.yaml", "path to config file")
	flag.Parse()

	cfg := getConfig(*cfgPath)

	ctx, cancel := context.WithCancel(context.Background())
	wsConnector := NewWsConnector(cfg, ctx)

	wg := sync.WaitGroup{}
	wg.Add(2)
	wsConnector.metrics.PrettyPrint = false

	go func() {
		wsConnector.metrics.Collect()
		wg.Done()
	}()

	go func() {
		wsConnector.RunWsScheduler()
		wg.Done()
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
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
