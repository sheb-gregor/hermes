//
package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lancer-kit/noble"
	"github.com/streadway/amqp"
	"gitlab.inn4science.com/ctp/hermes/config"
	"syreclabs.com/go/faker"
)

func main() {
	cfg := config.RabbitMQ{
		Host:     "127.0.0.1:5672",
		User:     noble.Secret{}.New("raw:user"),
		Password: noble.Secret{}.New("raw:p4s5w0rd"),
		Subs: []config.MqSubscription{
			{
				Exchange:     "ctp.notifications.prices",
				ExchangeType: "fanout",
				Queue:        "ctp.hermes.prices",
			},
			{
				Exchange:     "ctp.notifications.accounts",
				ExchangeType: "fanout",
				Queue:        "ctp.hermes.prices",
			},
			{
				Exchange:     "ctp.notifications.deposits",
				ExchangeType: "fanout",
				Queue:        "ctp.hermes.prices",
			},
		},
		ConsumerTag: "hermes_pub",
	}
	wg := sync.WaitGroup{}
	for _, sub := range cfg.Subs {
		wg.Add(1)
		go func(sub config.MqSubscription) {
			runPublishLoad(cfg.URL(), sub.Exchange, sub.ExchangeType)
			wg.Done()
		}(sub)
	}

	_ = faker.Address()
	wg.Wait()
}

func runPublishLoad(amqpURI, exchange, exchangeType string) {
	pub, err := newPublisher(amqpURI, exchange, exchangeType)
	if err != nil {
		log.Println(err)
		return
	}
	timer := time.NewTimer(time.Minute)
	send := time.NewTicker(time.Second)

	for {
		select {
		case <-timer.C:
			pub.Close()
			return

		case <-send.C:
			err = pub.Publish(exchange, "")
			if err != nil {
				log.Println(err)
				return
			}
		}
	}
}

type publisher struct {
	connection *amqp.Connection
	channel    *amqp.Channel
}

func newPublisher(amqpURI, exchange, exchangeType string) (*publisher, error) {
	var err error
	pubg := &publisher{}
	// This function dials, connects, declares, publishes, and tears down,
	// all in one go. In a real service, you probably want to maintain a
	// long-lived connection as state, and publish against that.

	log.Printf("dialing %q", amqpURI)
	pubg.connection, err = amqp.Dial(amqpURI)
	if err != nil {
		return nil, fmt.Errorf("Dial: %s", err)
	}

	log.Printf("got Connection, getting Channel")
	pubg.channel, err = pubg.connection.Channel()
	if err != nil {
		return nil, fmt.Errorf("channel: %s", err)
	}

	log.Printf("got Channel, declaring %q Exchange (%q)", exchangeType, exchange)
	if err := pubg.channel.ExchangeDeclare(
		exchange,
		exchangeType,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return nil, fmt.Errorf("exchange Declare: %s", err)
	}

	return pubg, nil
}

func (pubg *publisher) Close() {
	if err := pubg.channel.Close(); err != nil {
		log.Println(err)
	}

	if err := pubg.connection.Close(); err != nil {
		log.Println(err)
	}
	return
}

func (pubg *publisher) Publish(exchange, routingKey string) error {
	err := pubg.channel.Publish(
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			Headers: amqp.Table{
				"uuid":       "5526022f-3a80-4519-b5c4-0e4c29da742f",
				"visibility": faker.RandomChoice([]string{"broadcast", "direct"}),
				"event_type": exchange,
			},
			ContentType:     "application/json",
			ContentEncoding: "",
			Body:            []byte(fmt.Sprintf(`{"my_way":"%s"}`, faker.Lorem().Sentence(10))),
			DeliveryMode:    amqp.Transient,
			Priority:        0,
		},
	)

	if err != nil {
		return fmt.Errorf("exchange publish: %s", err)
	}
	return nil
}
