package main

import (
	"io/ioutil"
	"log"
	"sync"

	"github.com/google/uuid"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"gopkg.in/yaml.v2"
	"syreclabs.com/go/faker"

	"gitlab.inn4science.com/ctp/hermes/config"
)

func main() {
	cfg := getConfig("rabbit_emitter.config.yaml")

	exchangeRate := (len(cfg.RabbitMQ.Subs) * cfg.ConnNumber.ConnPercentage) / 100

	wg := sync.WaitGroup{}
	for i, mqSub := range cfg.RabbitMQ.Subs {
		wg.Add(1)
		go func(sub config.MqSubscription, i int) {
			mqSubmitter, err := NewRabbitSubmitter(cfg.RabbitMQ.URL())
			if err != nil {
				log.Fatalf("failed to create mq submitter: %s", err)
				return
			}

			if i < exchangeRate {
				mqSubmitter.exchange(sub, DirectExchange)
			} else {
				mqSubmitter.exchange(sub, BroadcastExchange)
			}
			wg.Done()
		}(mqSub, i)
	}
	wg.Wait()
}

func getConfig(path string) RabbitEmitterCfg {
	var cfg RabbitEmitterCfg

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

const (
	BroadcastExchange = "broadcast"
	DirectExchange    = "direct"
)

type rabbitEmitter struct {
	uri     string
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewRabbitSubmitter(uri string) (*rabbitEmitter, error) {
	var (
		err       error
		mqEmitter = &rabbitEmitter{
			uri: uri,
		}
	)
	log.Printf("connecting to amqp %s", uri)
	mqEmitter.conn, err = amqp.Dial(uri)
	if err != nil {
		log.Printf("failed amqp connection: %s", err)
		return nil, errors.Wrap(err, "failed to dial")
	}

	mqEmitter.channel, err = mqEmitter.conn.Channel()
	if err != nil {
		log.Printf("failed amqp channel conn: %s", err)
		return nil, errors.Wrap(err, "failed channel connection")
	}
	return mqEmitter, err
}

func (mqEmitter *rabbitEmitter) exchange(mqSub config.MqSubscription, exchangeType string) {
	err := mqEmitter.channel.ExchangeDeclare(
		mqSub.Exchange, mqSub.ExchangeType,
		true, false, false, false, nil,
	)
	if err != nil {
		log.Printf("failed to declare exchange %s with err: %s", mqSub.Exchange, err)
		return
	}
	log.Printf("declared the exchange %s with type %s", mqSub.Exchange, mqSub.ExchangeType)

	msg := faker.Lorem().Sentence(4)
	err = mqEmitter.publish(mqSub, msg, exchangeType)
	if err != nil {
		log.Printf("publish error: %s", err)
		return
	}
}

func (mqEmitter *rabbitEmitter) publish(sub config.MqSubscription, msg, exchangeType string) error {
	err := mqEmitter.channel.Publish(
		sub.Exchange,
		"",
		false,
		false,
		amqp.Publishing{
			Headers: amqp.Table{
				"uuid":       uuid.New().String(),
				"visibility": exchangeType,
			},
			ContentType:  "application/json",
			Body:         []byte(msg),
			DeliveryMode: amqp.Transient,
			Priority:     0,
		})
	if err != nil {
		return errors.Wrap(err, "failed to publish the message")
	}

	log.Printf("sended %s msg for %s sub exchange with %s exchange type", msg, sub.Exchange, exchangeType)
	return nil
}
