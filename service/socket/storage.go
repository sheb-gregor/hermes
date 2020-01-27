package socket

import (
	"sync"

	"github.com/pkg/errors"
)

type (
	syncStorage struct {
		*sync.Map
	}

	wsLiveSessions syncStorage
)

type wsSessionStorage syncStorage

func (w *wsSessionStorage) getByID(sessionID int64) (*Session, error) {
	value, ok := w.Load(sessionID)
	if !ok {
		return nil, errors.New("failed to load client data")
	}

	client, ok := value.(*Session)
	if !ok {
		return nil, errors.New("invalid value client cast")
	}
	return client, nil
}

type activeChannel syncStorage

func (a *activeChannel) getChannel(channel string) bool {
	value, ok := a.Load(channel)
	if !ok {
		return false
	}

	isActive, ok := value.(bool)
	if !ok {
		return false
	}
	return isActive
}

type activeEvent syncStorage

func (a *activeEvent) getSubscribeMap(channel string) map[string]bool {
	value, ok := a.Load(channel)
	if !ok {
		return nil
	}

	evenSubs, ok := value.(map[string]bool)
	if !ok {
		return nil
	}

	return evenSubs
}

type wsUsersSessions syncStorage

func (a wsUsersSessions) addSessionID(userUID string, connUID int64) {
	value, ok := a.Load(userUID)
	if !ok {
		value = map[int64]struct{}{}
	}

	evenSubs, _ := value.(map[int64]struct{})
	evenSubs[connUID] = struct{}{}
	a.Store(userUID, evenSubs)
}

func (a wsUsersSessions) rmSessionID(userUID string, connUID int64) {
	value, ok := a.Load(userUID)
	if !ok {
		return
	}

	evenSubs, _ := value.(map[int64]struct{})
	delete(evenSubs, connUID)

	a.Store(userUID, evenSubs)
}

func (a wsUsersSessions) getSessions(userUID string) map[int64]struct{} {
	value, ok := a.Load(userUID)
	if !ok {
		return map[int64]struct{}{}
	}

	evenSubs, _ := value.(map[int64]struct{})

	return evenSubs
}
