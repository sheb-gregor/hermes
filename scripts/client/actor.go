package main

import (
	"context"
	"fmt"
	"time"

	mc "hermes/metrics"

	"github.com/gorilla/websocket"
	"github.com/lancer-kit/uwe/v2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	BehaviorNormalAnon    = "normal_anon"
	BehaviorNormalUser    = "normal_user"
	BehaviorReconnectAnon = "reconnect_anon"
	BehaviorReconnectUser = "reconnect_user"
	BehaviorMonkey        = "monkey"
)

type ActorOpts struct {
	Behavior       string
	Token          string
	Origin         string
	Role           string
	DropDelay      int // in ms
	ReconnectDelay int // in ms
}
type Actor struct {
	log        *logrus.Entry
	opts       ActorOpts
	hermesURL  string
	metricsAdd func(key mc.MKey)
}

func NewActor(hermesURL string, config ActorOpts, log *logrus.Entry, metricsAdd func(key mc.MKey)) *Actor {
	return &Actor{
		opts:      config,
		hermesURL: hermesURL,
		log: log.WithFields(logrus.Fields{
			"behavior": config.Behavior,
			"role":     config.Role,
			"token":    config.Token,
		}),
		metricsAdd: metricsAdd,
	}
}

func (a *Actor) Init() error {
	return nil
}

func (a *Actor) Run(ctx uwe.Context) error {
	a.log.Info("run behavior")

	a.metricsAdd(mc.MKey("run_actor" + mc.Separator + a.opts.Behavior))

	var err error
	switch a.opts.Behavior {
	case BehaviorNormalAnon, BehaviorNormalUser:
		err = a.normalListenBehavior(ctx)
	case BehaviorReconnectAnon, BehaviorReconnectUser:
		err = a.reconnectBehavior(ctx)
	case BehaviorMonkey:
		err = a.monkeyBehavior(ctx)
	}
	if err != nil {
		a.log.WithError(err).Error("behavior failed")
	}
	return nil
}

func (a *Actor) normalListenBehavior(ctx context.Context) error {
	var err error
	client, err := NewClient(ctx, a.hermesURL, a.metricsAdd)
	if err != nil {
		return errors.Wrap(err, "can't create client")
	}

	done := make(chan struct{})
	events := make(chan Event)
	go func() {
		client.Listen(ctx, events)
		done <- struct{}{}
	}()

	defer func() {
		<-done
		err = client.Close()
		if err != nil {
			a.log.WithError(err).Error("client close failed")
		}
		return
	}()

	err = client.Handshake()
	if err != nil {
		return errors.Wrap(err, "can't send handshake call")
	}
	a.metricsAdd(mc.MKey("actor" + mc.Separator + "call" + mc.Separator + "handshake"))

	if a.opts.Behavior == BehaviorNormalUser || a.opts.Behavior == BehaviorReconnectUser {
		err = client.Authorize(ClientAuth{
			Token:  a.opts.Token,
			Origin: a.opts.Origin,
			Role:   a.opts.Role,
		})
		if err != nil {
			return errors.Wrap(err, "can't send authorize call")
		}
		a.metricsAdd(mc.MKey("actor" + mc.Separator + "call" + mc.Separator + "authorize"))
	}

	err = client.Subscribe("***", "***")
	if err != nil {
		return errors.Wrap(err, "can't send subscribe call")
	}
	a.metricsAdd(mc.MKey("actor" + mc.Separator + "call" + mc.Separator + "subscribe"))

	for {
		select {
		case event := <-events:
			switch event.Status {
			case StatusOk:
				a.metricsAdd(mc.MKey("actor" + mc.Separator + "receive" + mc.Separator + "ok"))
				a.metricsAdd(mc.MKey("actor" + mc.Separator + "receive" +
					mc.Separator + "ok" + mc.Separator + a.opts.Token))

				a.metricsAdd(mc.MKey("actor" + mc.Separator + "receive" +
					mc.Separator + event.Message.Event + mc.Separator + a.opts.Token))

			case StatusError:
				a.metricsAdd(mc.MKey("actor" + mc.Separator + "receive" + mc.Separator + "error"))
				a.metricsAdd(mc.MKey("actor" + mc.Separator + "receive" +
					mc.Separator + "error" + mc.Separator + a.opts.Token))

			case StatusTerminate, StatusNormalStop:
				return nil
			}

		case <-ctx.Done():
			return nil
		}
	}

}

func (a *Actor) reconnectBehavior(ctx uwe.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			a.metricsAdd(mc.MKey("actor" + mc.Separator + "reconnect"))
			a.metricsAdd(mc.MKey("actor" + mc.Separator + "reconnect" + mc.Separator + a.opts.Token))

			newCtx, _ := context.WithTimeout(ctx, time.Duration(a.opts.ReconnectDelay)*time.Millisecond)
			err := a.normalListenBehavior(newCtx)
			if err != nil {
				a.log.WithError(err).Error("normalListenBehavior iteration failed")
			}

		}
	}
}

func (a *Actor) monkeyBehavior(ctx uwe.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			a.metricsAdd(mc.MKey("actor" + mc.Separator + "runDrop"))
			a.metricsAdd(mc.MKey("actor" + mc.Separator + "runDrop" + mc.Separator + a.opts.Token))
			err := a.connectAndDrop(context.Background())
			if err != nil {
				a.log.WithError(err).Error("connectAndDrop iteration failed")
			}
		}
	}
}

func (a *Actor) connectAndDrop(ctx context.Context) error {
	var err error
	client, err := NewClient(ctx, a.hermesURL, a.metricsAdd)
	if err != nil {
		return errors.Wrap(err, "can't create client")
	}

	defer func() {
		rec := recover()
		if rec != nil {
			a.log.WithField("recover ", fmt.Sprintf("%+v", rec)).
				Error("caught panic")
		}
	}()

	err = client.Handshake()
	if err != nil {
		return errors.Wrap(err, "can't send handshake call")
	}
	a.metricsAdd(mc.MKey("actor" + mc.Separator + "call" + mc.Separator + "handshake"))

	if time.Now().Unix()%3 == 0 {
		err = client.Authorize(ClientAuth{
			Token:  a.opts.Token,
			Origin: a.opts.Origin,
			Role:   a.opts.Role,
		})
		if err != nil {
			return errors.Wrap(err, "can't send authorize call")
		}
		a.metricsAdd(mc.MKey("actor" + mc.Separator + "call" + mc.Separator + "authorize"))
	}

	err = client.Subscribe("***", "***")
	if err != nil {
		return errors.Wrap(err, "can't send subscribe call")
	}
	a.metricsAdd(mc.MKey("actor" + mc.Separator + "call" + mc.Separator + "subscribe"))

	time.Sleep(time.Duration(a.opts.DropDelay) * time.Millisecond)

	*client.ws = websocket.Conn{}
	a.metricsAdd(mc.MKey("actor" + mc.Separator + "dropClient"))
	return nil
}
