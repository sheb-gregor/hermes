package main

import (
	"io/ioutil"
	"log"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/metrics"
	"syreclabs.com/go/faker"
)

const (
	broadcastExchange = "broadcast"
	directExchange    = "direct"
)

type rabbitEmitter struct {
	uri string
	cfg RabbitEmitterCfg

	metrics *metrics.SafeMetrics
}

func NewRabbitEmitter(uri string, cfg RabbitEmitterCfg) *rabbitEmitter {
	return &rabbitEmitter{
		uri:     uri,
		cfg:     cfg,
		metrics: new(metrics.SafeMetrics).New(),
	}
}

func (e *rabbitEmitter) SaveMetrics() {
	data, err := e.metrics.MarshalJSON()
	if err != nil {
		log.Fatalf("failed to marshal the metrics: %s", err)
	}
	err = ioutil.WriteFile("emitter_metrics_report.json", data, 0644)
	if err != nil {
		log.Printf("failed to write the metrics: %s", err)
	}
}

type rabbitPublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewRabbitPublisher(uri string, cfg RabbitEmitterCfg) (*rabbitPublisher, error) {
	var err error

	mqEmitter := new(rabbitPublisher)

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

	err = mqEmitter.channel.ExchangeDeclare(
		cfg.RabbitMQ.Common.Exchange, cfg.RabbitMQ.Common.ExchangeType,
		true, false, false, false, nil,
	)
	if err != nil {
		log.Printf("failed to declare msg publisher with err: %s", err)
	}
	// log.Printf("declared the publishMessage %s with type %s",
	// 	cfg.RabbitMQ.Common.Exchange, cfg.RabbitMQ.Common.ExchangeType)
	return mqEmitter, err
}

func (r *rabbitPublisher) publishMessage(mqSub config.Exchange,
	exchangeType string, metricsCollector *metrics.SafeMetrics) error {
	metricsCollector.Add(metrics.MKey("exchangeType." + exchangeType))

	emitterChannelNameMKey := metrics.MKey(exchangeType + "." + mqSub.Exchange)
	msg := `{"t":"` + faker.Lorem().Sentence(4) + `"}`

	err := r.channel.Publish(
		mqSub.Exchange,
		"",
		false,
		false,
		amqp.Publishing{
			Headers: amqp.Table{
				"uuid":       "someSpecialAuthToken",
				"visibility": exchangeType,
				"event_type": mqSub.Exchange,
			},
			ContentType:  "application/json",
			Body:         []byte(msg),
			DeliveryMode: amqp.Transient,
			Priority:     0,
		})
	if err != nil {
		return errors.Wrap(err, "failed to publish the message")
	}

	metricsCollector.Add(emitterChannelNameMKey)

	// log.Printf("sended %s msg for %s sub publishMessage with %s publishMessage type",
	// 	msg, mqSub.Exchange, exchangeType)
	return nil
}

func (r *rabbitPublisher) Close() {
	if err := r.channel.Close(); err != nil {
		log.Println(err)
	}

	if err := r.conn.Close(); err != nil {
		log.Println(err)
	}
}
