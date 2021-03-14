package ws

import (
	"sync"

	"hermes/models"
)

type SessionStorage interface {
	AddSession(id int64, obj *Session)
	AddUserSession(userID string, info models.SessionInfo)

	GetSession(id int64) *Session
	GetSessions() map[int64]*Session
	GetSessionsCount() int64

	GetUserSessions(userID string) map[int64]models.SessionInfo
	ListUsersSessions() map[string]map[int64]models.SessionInfo

	ForEach(action func(id int64, session *Session))

	RMSession(id int64) *Session
	RMSessions(callback func(id int64, session *Session))
}

type sessionDB struct {
	sync.RWMutex
	data         map[int64]*Session
	userSessions map[string]map[int64]struct{}
}

func newSessionStorage() SessionStorage {
	return &sessionDB{
		data:         map[int64]*Session{},
		userSessions: map[string]map[int64]struct{}{},
	}
}

func (storage *sessionDB) AddSession(id int64, obj *Session) {
	storage.Lock()
	defer storage.Unlock()

	storage.data[id] = obj

}

func (storage *sessionDB) GetSession(id int64) *Session {
	storage.RLock()
	defer storage.RUnlock()
	return storage.data[id]
}

func (storage *sessionDB) GetSessions() map[int64]*Session {
	storage.RLock()
	defer storage.RUnlock()
	return storage.data
}

func (storage *sessionDB) UpdateSessionInfo(id int64, info models.SessionInfo) {
	storage.Lock()
	defer storage.Unlock()

}

func (storage *sessionDB) GetSessionsCount() int64 {
	storage.RLock()
	defer storage.RUnlock()
	return int64(len(storage.data))
}

func (storage *sessionDB) AddUserSession(userID string, info models.SessionInfo) {
	storage.Lock()
	defer storage.Unlock()

	storage.data[info.ID].info = info
	storage.addUserSessionID(info.ID, userID)
}

func (storage *sessionDB) RMSession(id int64) *Session {
	storage.Lock()
	defer storage.Unlock()
	session, ok := storage.data[id]
	if !ok {
		return nil
	}

	storage.rmUserSession(session.info.UserID, id)
	delete(storage.data, id)

	return session
}

func (storage *sessionDB) RMSessions(callback func(id int64, session *Session)) {
	storage.Lock()
	defer storage.Unlock()

	for id := range storage.data {
		session := storage.data[id]

		storage.rmUserSession(session.info.UserID, id)
		delete(storage.data, id)
		callback(id, session)
	}
}

func (storage *sessionDB) GetUserSessions(userID string) map[int64]models.SessionInfo {
	storage.RLock()
	defer storage.RUnlock()

	sessions, ok := storage.userSessions[userID]
	if !ok {
		return map[int64]models.SessionInfo{}
	}

	result := make(map[int64]models.SessionInfo, len(sessions))
	for id := range sessions {
		result[id] = storage.data[id].info
	}

	return result
}

func (storage *sessionDB) ListUsersSessions() map[string]map[int64]models.SessionInfo {
	storage.RLock()
	defer storage.RUnlock()
	list := make(map[string]map[int64]models.SessionInfo, len(storage.userSessions))

	for userID, sessions := range storage.userSessions {
		list[userID] = make(map[int64]models.SessionInfo, len(sessions))
		for id := range sessions {
			list[userID][id] = storage.data[id].info
		}

	}

	return list
}

func (storage *sessionDB) ForEach(action func(id int64, session *Session)) {
	storage.RLock()
	defer storage.RUnlock()

	for i, session := range storage.data {
		action(i, session)
	}
}

func (storage *sessionDB) addUserSessionID(sessionID int64, userID string) {
	sessions, ok := storage.userSessions[userID]
	if !ok {
		sessions = map[int64]struct{}{}
	}

	sessions[sessionID] = struct{}{}
	storage.userSessions[userID] = sessions
}

func (storage *sessionDB) rmUserSession(userID string, sessionID int64) {
	sessions, ok := storage.userSessions[userID]
	if !ok {
		return
	}

	delete(sessions, sessionID)
	if len(sessions) == 0 {
		delete(storage.userSessions, userID)
		return
	}

	storage.userSessions[userID] = sessions
}
