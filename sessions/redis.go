package sessions

import (
	"encoding/json"

	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"gitlab.inn4science.com/ctp/hermes/config"
)

const (
	commandGet    = "GET"
	commandSet    = "SET"
	commandPing   = "PING"
	commandExpire = "EX"
	replyPong     = "PONG"
)

type RedisStorage struct {
	cfg  config.RedisConf
	pool *redis.Pool
}

// NewStorage returns initialized instance of the `Repo`.
func NewRedisStorage(cfg config.RedisConf) (*RedisStorage, error) {
	if cfg.DevMode {
		return &RedisStorage{cfg: cfg, pool: nil}, nil
	}

	pool := NewPool(cfg)
	_, err := pool.Dial()
	if err != nil {
		return nil, errors.Wrap(err, "invalid redis configuration url")
	}

	return &RedisStorage{cfg: cfg, pool: pool}, nil
}

func (s *RedisStorage) CheckConn() error {
	conn := s.pool.Get()
	defer conn.Close()

	reply, err := redis.String(conn.Do(commandPing))
	if err != nil {
		s.pool.Close()
		return errors.Wrap(err, "connection failed")
	}

	if reply != replyPong {
		return errors.New("failed to receive ping response from redis")
	}

	return nil
}

func (s *RedisStorage) CloseConnection() error {
	return s.pool.Close()
}

func (s *RedisStorage) GetSession(key string) (*Session, error) {
	if s.cfg.DevMode {
		return &Session{UserID: key, Active: true}, nil
	}

	conn := s.pool.Get()
	defer conn.Close()

	rawSensitive, err := redis.Bytes(conn.Do(commandGet, key))
	if err != nil && err.Error() == redis.ErrNil.Error() {
		return nil, err
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to perform %s command, data wasn't retrieved", commandGet)
	}

	session := new(Session)
	err = json.Unmarshal(rawSensitive, session)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal raw value into target")
	}

	return session, nil
}

// SaveAsJSON saves any value as json object with provided key
func (s *RedisStorage) SaveAsJSON(key string, value interface{}, ttl int64) error {
	if s.cfg.DevMode {
		return nil
	}

	conn := s.pool.Get()
	defer conn.Close()

	sensitiveMarshalled, err := json.Marshal(value)
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}

	_, err = conn.Do(commandSet, key, sensitiveMarshalled, commandExpire, ttl)
	if err != nil {
		return errors.Wrapf(err, "failed to perform %s command, data wasn't saved", commandSet)
	}

	return nil
}
