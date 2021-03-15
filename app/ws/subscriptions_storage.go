package ws

import "sync"

type SubscriptionsStore interface {
	AddSubscription(channel, event string)
	IsSubscribed(channel, event string) bool
	RmSubscription(channel string)
	MuteEvent(event string)
	UnmuteEvent(event string)
}

type Subscriptions struct {
	sync.RWMutex
	channels    map[string]struct{}
	events      map[string]map[string]struct{}
	mutedEvents map[string]struct{}
}

func NewSubscriptions() *Subscriptions {
	return &Subscriptions{
		channels:    map[string]struct{}{},
		events:      map[string]map[string]struct{}{},
		mutedEvents: map[string]struct{}{},
	}
}

func (subStore *Subscriptions) AddSubscription(channel, event string) {
	subStore.Lock()
	defer subStore.Unlock()

	if channel == "" {
		return
	}

	if event == "" {
		event = WildcardSubscription
	}

	subStore.channels[channel] = struct{}{}

	eventSubs := subStore.events[channel]
	if eventSubs == nil {
		eventSubs = make(map[string]struct{})
	}

	eventSubs[event] = struct{}{}
	subStore.events[channel] = eventSubs

	delete(subStore.mutedEvents, event)
}

func (subStore *Subscriptions) IsSubscribed(channel, event string) bool {
	subStore.RLock()
	defer subStore.RUnlock()

	if channel == EvCache || channel == EvStatusChannel {
		return true
	}

	if _, ok := subStore.mutedEvents[event]; ok {
		return false
	}

	if _, ok := subStore.channels[WildcardSubscription]; ok {
		eventSubs := subStore.events[WildcardSubscription]
		if eventSubs == nil {
			return true
		}
		_, hasEventSub := eventSubs[event]
		_, hasWildcardSub := eventSubs[WildcardSubscription]
		return hasEventSub || hasWildcardSub
	}

	_, chanSub := subStore.channels[channel]
	eventSubs := subStore.events[channel]
	if eventSubs == nil {
		return false
	}

	_, hasEventSub := eventSubs[event]
	_, hasWildcardSub := eventSubs[WildcardSubscription]

	eventSub := hasEventSub || hasWildcardSub
	return chanSub && eventSub
}

func (subStore *Subscriptions) RmSubscription(channel string) {
	subStore.Lock()

	delete(subStore.channels, channel)
	delete(subStore.events, channel)

	subStore.Unlock()
}

func (subStore *Subscriptions) MuteEvent(event string) {
	subStore.Lock()
	subStore.mutedEvents[event] = struct{}{}
	subStore.Unlock()
}

func (subStore *Subscriptions) UnmuteEvent(event string) {
	subStore.Lock()
	delete(subStore.mutedEvents, event)
	subStore.Unlock()
}
