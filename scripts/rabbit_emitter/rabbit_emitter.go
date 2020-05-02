package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/lancer-kit/uwe/v2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	mc "gitlab.inn4science.com/ctp/hermes/metrics"
	"gitlab.inn4science.com/ctp/hermes/models"
	"syreclabs.com/go/faker"
)

type emitterCfg struct {
	uri string

	authFormat   TokenFormat
	distribution Distribution
	tickPeriod   uint
}

type emitter struct {
	emitterCfg

	conn    *amqp.Connection
	channel *amqp.Channel

	log        *logrus.Entry
	metricsAdd func(key mc.MKey)
}

func NewRabbitEmitter(cfg emitterCfg, mcAdd func(key mc.MKey)) *emitter {
	return &emitter{
		emitterCfg: cfg,
		metricsAdd: mcAdd,
	}
}

func (em *emitter) Init() (err error) {
	em.conn, err = amqp.Dial(em.uri)
	if err != nil {
		return errors.Wrap(err, "failed to dial")
	}

	em.channel, err = em.conn.Channel()
	if err != nil {
		return errors.Wrap(err, "failed channel connection")
	}
	return nil
}

func (em *emitter) Run(ctx uwe.Context) error {
	ticker := time.NewTicker(time.Duration(em.tickPeriod) * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			return em.Close()
		case <-ticker.C:
			if em.distribution.Broadcast {
				em.publishMessage("")
			}

			if !em.distribution.DirectRandom {
				for i := em.authFormat.From; i < em.authFormat.To; i++ {
					em.publishMessage(fmt.Sprintf(em.authFormat.Format, i))
				}
			} else {
				rand.Seed(time.Now().UnixNano())
				min := int(em.authFormat.From)
				max := int(em.authFormat.To)
				i := rand.Intn(max-min+1) + min
				em.publishMessage(fmt.Sprintf(em.authFormat.Format, i))
			}
		}
	}
}

func (em *emitter) publishMessage(uid string) {
	msg := `{"phrase":"` + faker.Lorem().Sentence(4) + `"}`
	headers := amqp.Table{
		models.RHeaderEvent:      em.distribution.Queue,
		models.RHeaderVisibility: models.VisibilityBroadcast,
	}
	mKey := models.VisibilityBroadcast + mc.Separator + em.distribution.Queue

	if uid != "" {
		headers[models.RHeaderVisibility] = models.VisibilityDirect
		headers[models.RHeaderUUID] = uid
		mKey = models.VisibilityDirect + mc.Separator + em.distribution.Queue + mc.Separator + uid
	}

	err := em.channel.Publish(
		em.distribution.Exchange,
		em.distribution.Queue,
		false,
		false,
		amqp.Publishing{
			Headers:      headers,
			ContentType:  "application/json",
			Body:         []byte(msg),
			DeliveryMode: amqp.Transient,
			Priority:     0,
		})
	if err != nil {
		log.Print("[ERROR] ", "failed to publish message:", err.Error())
	}

	em.metricsAdd(mc.NewMKey(mKey))
}

func (em *emitter) Close() error {
	if err := em.channel.Close(); err != nil {
		return err
	}

	if err := em.conn.Close(); err != nil {
		return err
	}

	return nil
}
