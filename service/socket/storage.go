package socket

import (
	"sync"

	"github.com/pkg/errors"
	"gitlab.inn4science.com/ctp/hermes/models"
)

type syncStorage struct{ *sync.Map }

type wsLiveSessions syncStorage

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

type wsUsersSessions struct {
	sync.RWMutex
	sync.Map
}

func (a *wsUsersSessions) addSessionID(userUID string, session models.SessionInfo) {
	a.Lock()
	defer a.Unlock()

	value, ok := a.Load(userUID)
	if !ok {
		value = map[int64]struct{}{}
	}

	sessions, ok := value.(map[int64]models.SessionInfo)
	if !ok {
		sessions = map[int64]models.SessionInfo{}
	}
	sessions[session.ID] = session
	a.Store(userUID, sessions)
}

func (a *wsUsersSessions) rmSessionID(userUID string, sessionID int64) {
	a.Lock()
	defer a.Unlock()

	value, ok := a.Load(userUID)
	if !ok {
		return
	}

	sessions, _ := value.(map[int64]models.SessionInfo)
	delete(sessions, sessionID)

	a.Store(userUID, sessions)
}

func (a *wsUsersSessions) getSessions(userUID string) map[int64]models.SessionInfo {
	a.RLock()
	defer a.RUnlock()

	value, ok := a.Load(userUID)
	if !ok {
		return map[int64]models.SessionInfo{}
	}

	sessions, _ := value.(map[int64]models.SessionInfo)
	return sessions
}
