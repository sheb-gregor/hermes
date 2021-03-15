package app

import (
	"context"
	"sync"

	"hermes/app/ws"
	"hermes/config"
	"hermes/metrics"
	"hermes/models"

	"github.com/lancer-kit/uwe/v2"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/streadway/amqp"
)

type RabbitConsumer struct {
	config  config.RabbitMQ
	logger  zerolog.Logger
	devMode bool
	wg      *sync.WaitGroup

	conn    *amqp.Connection
	channel *amqp.Channel

	queueCancelers  map[string]context.CancelFunc
	queueManagement chan models.ManageQueue
	outBus          chan<- *ws.Event
}

func NewRabbitConsumer(logger zerolog.Logger, configuration config.RabbitMQ,
	outBus chan<- *ws.Event) (uwe.Worker, chan<- models.ManageQueue) {
	qm := make(chan models.ManageQueue)

	return &RabbitConsumer{
		logger:          logger,
		devMode:         logger.GetLevel() == zerolog.TraceLevel,
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
	worker.conn, err = amqp.Dial(rabbitCfg.Auth.URL())
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

func (worker *RabbitConsumer) ensureExchange(mqSub config.Exchange) error {
	err := worker.channel.ExchangeDeclare(
		mqSub.Exchange, mqSub.ExchangeType,
		true, false, false, false, nil,
	)
	if err != nil {
		return errors.Wrap(err, "failed to declare exchange - "+mqSub.Exchange)
	}
	return nil
}

func (worker *RabbitConsumer) ensureQueue(mqSub config.Exchange) error {
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

func (worker *RabbitConsumer) runQueueSub(ctx context.Context, sub config.Exchange, out chan amqp.Delivery) {
	if _, ok := worker.queueCancelers[sub.Queue]; ok {
		return
	}

	subCtx, cancel := context.WithCancel(ctx)
	worker.queueCancelers[sub.Queue] = cancel

	worker.wg.Add(1)

	go func(subscription config.Exchange) {
		defer worker.wg.Done()

		if err := worker.startConsumingRoutine(subCtx, subscription, out); err != nil {
			worker.logger.Error().Str("queue", subscription.Queue).
				Str("exchange", subscription.Exchange).
				Err(err).Msg("failed to subscribe")
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
			logger := worker.logger.With().Fields(map[string]interface{}{
				"routing_key":  message.RoutingKey,
				"consumer_tag": message.ConsumerTag,
				"exchange":     message.Exchange,
			}).Logger()

			if worker.devMode {
				logger.
					Trace().Fields(map[string]interface{}{
					"delivery_tag": message.DeliveryTag,
					"message_id":   message.MessageId,
					"content_type": message.ContentType,
					"event_kind":   message.Headers[models.RHeaderEvent],
					"visibility":   message.Headers[models.RHeaderVisibility],
					"body":         string(message.Body),
				}).Msg("received a deliveries from consumer")
			}

			params, err := models.ParseRabbitHeader(message)
			if err != nil {
				logger.Warn().Err(err).Msg("invalid header")
			}

			if params.Visibility == models.VisibilityInternal {
				continue
			}

			worker.outBus <- &ws.Event{
				Kind: ws.EKMessage,
				Message: &models.Message{
					Meta: models.MessageMeta{
						Broadcast: params.Broadcast,
						Role:      params.Role,
						UserUID:   params.UUID,
						TTL:       params.CacheTTL,
					},

					Channel: message.Exchange,
					Event:   params.Event,
					Data:    map[string]interface{}{params.Event: string(message.Body)},
				},
			}

			metrics.Inc(config.IncomingRabbitMessages)

		case <-wCtx.Done():
			cancel()
			worker.wg.Wait()

			worker.logger.Info().Msg("Receive exit code, stop all consumers")
			if err := worker.channel.Close(); err != nil {
				worker.logger.Warn().Err(err).Msg("fail when try to close channel")
			}
			if err := worker.conn.Close(); err != nil {
				worker.logger.Warn().Err(err).Msg("fail when try to close connection")
			}

			return nil
		}
	}
}

func (worker *RabbitConsumer) startConsumingRoutine(ctx context.Context,
	sub config.Exchange, out chan amqp.Delivery) error {
	logger := worker.logger.With().
		Str("queue", sub.Queue).
		Str("exchange", sub.Exchange).Logger()

	if err := worker.ensureQueue(sub); err != nil {
		return errors.Wrap(err, "failed to ensure queue")
	}

	consume, err := worker.channel.Consume(
		sub.Queue, worker.config.Auth.GetConsumerTag(sub.Queue),
		true, false, false, false, nil)
	if err != nil {
		return errors.Wrap(err, "failed to register a consumer")
	}

	logger.Info().Msg("Run consumer loop")
	for {
		select {
		case message := <-consume:
			if message.Body == nil {
				continue
			}

			if worker.devMode {
				logger.Trace().Fields(map[string]interface{}{
					"delivery_tag": message.DeliveryTag,
					"exchange":     message.Exchange,
					"routing_key":  message.RoutingKey,
					"consumer_tag": message.ConsumerTag,
					"message_id":   message.MessageId,
					"content_type": message.ContentType,
					"visibility":   message.Headers["visibility"],
					"uuid":         message.Headers["uuid"],
					"body":         string(message.Body),
				}).Msg("received a new message from queue")
			}

			out <- message

		case <-ctx.Done():
			logger.Info().Msg("Receive exit code, stop working")
			return nil
		}
	}
}
