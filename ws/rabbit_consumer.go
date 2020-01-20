package ws

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/lancer-kit/uwe/v2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/models"
	"gitlab.inn4science.com/ctp/hermes/ws/socket"
)

const (
	RHeaderUUID       = "uuid"
	RHeaderVisibility = "visibility"
	RHeaderEvent      = "event_type"

	VisibilityBroadcast = "broadcast"
	VisibilityDirect    = "direct"
)

type RabbitConsumer struct {
	config config.RabbitMQ
	logger *logrus.Entry

	conn    *amqp.Connection
	channel *amqp.Channel

	outBus chan<- *socket.Event
}

func NewRabbitConsumer(logger *logrus.Entry, configuration config.RabbitMQ, outBus chan<- *socket.Event) uwe.Worker {
	return &RabbitConsumer{
		logger: logger,
		config: configuration,
		outBus: outBus,
	}
}

func (worker *RabbitConsumer) Init() error {
	var err error
	rabbitCfg := worker.config
	worker.conn, err = amqp.Dial(rabbitCfg.URL())
	if err != nil {
		return errors.Wrap(err, "failed to connect to RabbitMQ")
	}

	worker.channel, err = worker.conn.Channel()
	if err != nil {
		return errors.Wrap(err, "failed to connect to RabbitMQ")
	}

	for _, mqSub := range rabbitCfg.Subs {
		err = worker.channel.ExchangeDeclare(
			mqSub.Exchange, mqSub.ExchangeType,
			true, false, false, false, nil,
		)
		if err != nil {
			return errors.Wrap(err, "failed to declare exchange - "+mqSub.Exchange)
		}

		_, err = worker.channel.QueueDeclare(
			mqSub.Queue, false, false, false, false, nil)
		if err != nil {
			return errors.Wrap(err, "failed to declare a queue")
		}

		err = worker.channel.QueueBind(
			mqSub.Queue, mqSub.RoutingKey, mqSub.Exchange, false, nil)
		if err != nil {
			return errors.Wrap(err, "failed to bind queue: %s")
		}
	}

	return nil
}

// nolint:funlen
func (worker *RabbitConsumer) Run(wCtx uwe.Context) error {
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(wCtx)
	deliveries := make(chan amqp.Delivery, len(worker.config.Subs))

	for _, sub := range worker.config.Subs {
		wg.Add(1)
		logger := worker.logger.
			WithField("queue", sub.Queue).
			WithField("exchange", sub.Exchange)

		go func(subscription config.MqSubscription) {
			defer wg.Done()

			if err := worker.runSub(ctx, subscription, deliveries); err != nil {
				logger.WithError(err).Error("failed to subscribe")
				return
			}
		}(sub)
	}

	for {
		select {
		case message := <-deliveries:
			if message.Body == nil {
				continue
			}

			worker.logger.
				WithFields(logrus.Fields{
					"delivery_tag": message.DeliveryTag,
					"exchange":     message.Exchange,
					"routing_key":  message.RoutingKey,
					"consumer_tag": message.ConsumerTag,
					"message_id":   message.MessageId,
					"content_type": message.ContentType,
					"visibility":   message.Headers["visibility"],
					"uuid":         message.Headers["uuid"],
					"body":         string(message.Body),
				}).
				Trace("received a deliveries from consumer")

			visibility, ok := message.Headers[RHeaderVisibility].(string)
			if !ok {
				// log.Warn
				continue
			}

			uuid, ok := message.Headers[RHeaderUUID].(string)
			if !ok && visibility == VisibilityDirect {
				// log.Warn
				continue
			}

			event, ok := message.Headers[RHeaderEvent].(string)
			if !ok {
				// log.Warn
				continue
			}

			worker.outBus <- &socket.Event{
				Kind: socket.EKMessage,
				Message: &models.Message{
					Broadcast: visibility == VisibilityBroadcast,
					Channel:   message.Exchange,
					Event:     event,
					UserUID:   uuid,
					Data:      map[string]interface{}{event: json.RawMessage(message.Body)},
				},
			}
		case <-wCtx.Done():
			cancel()
			wg.Wait()
			worker.logger.Info("Receive exit code, stop all consumers")
			return nil
		}
	}
}

func (worker *RabbitConsumer) runSub(ctx context.Context, sub config.MqSubscription, out chan amqp.Delivery) error {
	logger := worker.logger.
		WithField("queue", sub.Queue).
		WithField("exchange", sub.Exchange)

	q, err := worker.channel.QueueDeclare(
		sub.Queue, false, false, false, false, nil)
	if err != nil {
		logger.WithError(err).Error("failed to declare a queue")
		return errors.Wrap(err, "failed to declare a queue")
	}

	consume, err := worker.channel.Consume(
		q.Name, worker.config.GetConsumerTag(q.Name), true, false, false, false, nil)
	if err != nil {
		logger.WithError(err).Error("failed to register a consumer")
		return errors.Wrap(err, "failed to register a consumer")
	}

	logger.Info("Run consumer loop")
	for {
		select {
		case message := <-consume:
			if message.Body == nil {
				continue
			}
			logger.WithFields(logrus.Fields{
				"delivery_tag": message.DeliveryTag,
				"exchange":     message.Exchange,
				"routing_key":  message.RoutingKey,
				"consumer_tag": message.ConsumerTag,
				"message_id":   message.MessageId,
				"content_type": message.ContentType,
				"visibility":   message.Headers["visibility"],
				"uuid":         message.Headers["uuid"],
				"body":         string(message.Body),
			}).
				Trace("received a new message from queue")

			out <- message

		case <-ctx.Done():
			if err := worker.channel.Close(); err != nil {
				logger.WithError(err).Warn("fail when try to close channel")
			}

			if err := worker.conn.Close(); err != nil {
				logger.WithError(err).Warn("fail when try to close channel")
			}

			logger.Info("Receive exit code, stop working")
			return nil
		}
	}
}
