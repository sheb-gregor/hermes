package service

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
	"gitlab.inn4science.com/ctp/hermes/service/socket"
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
	wg     *sync.WaitGroup

	conn    *amqp.Connection
	channel *amqp.Channel

	queueCancelers  map[string]context.CancelFunc
	queueManagement chan models.ManageQueue
	outBus          chan<- *socket.Event
}

func NewRabbitConsumer(logger *logrus.Entry, configuration config.RabbitMQ,
	outBus chan<- *socket.Event) (uwe.Worker, chan<- models.ManageQueue) {
	qm := make(chan models.ManageQueue)
	return &RabbitConsumer{
		logger:          logger,
		config:          configuration,
		outBus:          outBus,
		queueManagement: qm,
		queueCancelers:  map[string]context.CancelFunc{},
		wg:              &sync.WaitGroup{},
	}, qm
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
		err = worker.ensureExchange(mqSub)
		if err != nil {
			return err
		}
	}

	return worker.ensureExchange(rabbitCfg.Common)
}

func (worker *RabbitConsumer) ensureExchange(mqSub config.MqSubscription) error {
	err := worker.channel.ExchangeDeclare(
		mqSub.Exchange, mqSub.ExchangeType,
		true, false, false, false, nil,
	)
	if err != nil {
		return errors.Wrap(err, "failed to declare exchange - "+mqSub.Exchange)
	}
	return nil
}

func (worker *RabbitConsumer) ensureQueue(mqSub config.MqSubscription) error {
	_, err := worker.channel.QueueDeclare(
		mqSub.Queue, false, false, false, false, nil)
	if err != nil {
		return errors.Wrap(err, "failed to declare a queue")
	}

	routingKey := mqSub.RoutingKey
	if routingKey == "" {
		routingKey = mqSub.Queue
	}

	err = worker.channel.QueueBind(
		mqSub.Queue, routingKey, mqSub.Exchange, false, nil)
	if err != nil {
		return errors.Wrap(err, "failed to bind queue: %s")
	}

	return nil
}

func (worker *RabbitConsumer) runQueueSub(ctx context.Context, sub config.MqSubscription, out chan amqp.Delivery) {
	if _, ok := worker.queueCancelers[sub.Queue]; ok {
		return
	}

	subCtx, cancel := context.WithCancel(ctx)
	worker.queueCancelers[sub.Queue] = cancel

	worker.wg.Add(1)
	logger := worker.logger.
		WithField("queue", sub.Queue).
		WithField("exchange", sub.Exchange)

	go func(subscription config.MqSubscription) {
		defer worker.wg.Done()

		if err := worker.startConsumingRoutine(subCtx, subscription, out); err != nil {
			logger.WithError(err).Error("failed to subscribe")
			return
		}
	}(sub)

}

// nolint:funlen
func (worker *RabbitConsumer) Run(wCtx uwe.Context) error {
	ctx, cancel := context.WithCancel(wCtx)
	deliveries := make(chan amqp.Delivery, len(worker.config.Subs))

	for _, sub := range worker.config.Subs {
		worker.runQueueSub(ctx, sub, deliveries)
	}

	for {
		select {
		case qm := <-worker.queueManagement:
			switch qm.Action {
			case models.ActionAddQueue:
				worker.runQueueSub(ctx, worker.config.GetCommonSub(qm.Queue), deliveries)
			case models.ActionRmQueue:
				if qCancel, ok := worker.queueCancelers[qm.Queue]; ok {
					qCancel()
				}
			}

		case message := <-deliveries:
			if message.Body == nil {
				continue
			}
			logger := worker.logger.WithFields(logrus.Fields{
				"routing_key":  message.RoutingKey,
				"consumer_tag": message.ConsumerTag,
				"exchange":     message.Exchange})

			logger.
				WithFields(logrus.Fields{
					"delivery_tag": message.DeliveryTag,
					"message_id":   message.MessageId,
					"content_type": message.ContentType,
					"visibility":   message.Headers[RHeaderVisibility],
					"uuid":         message.Headers[RHeaderUUID],
					"body":         string(message.Body),
				}).
				Trace("received a deliveries from consumer")

			visibility, ok := message.Headers[RHeaderVisibility].(string)
			if !ok {
				logger.Warn("message don't have RHeaderVisibility")
				continue
			}

			uuid, ok := message.Headers[RHeaderUUID].(string)
			if !ok && visibility == VisibilityDirect {
				logger.Warn("message don't have RHeaderUUID")
				continue
			}

			event, ok := message.Headers[RHeaderEvent].(string)
			if !ok {
				logger.Warn("message don't have RHeaderEvent")
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
			worker.wg.Wait()
			worker.logger.Info("Receive exit code, stop all consumers")
			return nil
		}
	}
}

func (worker *RabbitConsumer) startConsumingRoutine(ctx context.Context,
	sub config.MqSubscription, out chan amqp.Delivery) error {
	logger := worker.logger.
		WithField("queue", sub.Queue).
		WithField("exchange", sub.Exchange)

	if err := worker.ensureQueue(sub); err != nil {
		logger.WithError(err).Error("failed to ensure queue")
		return errors.Wrap(err, "failed to ensure queue")
	}

	consume, err := worker.channel.Consume(
		sub.Queue, worker.config.GetConsumerTag(sub.Queue), true, false, false, false, nil)
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
