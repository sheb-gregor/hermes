package cache

import (
	"encoding/json"

	"hermes/config"
	"hermes/models"

	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
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

	stats BucketStats
}

// NewStorage returns initialized instance of the `Repo`.
func NewRedisStorage(cfg config.RedisConf, stats BucketStats) (*RedisStorage, error) {
	if cfg.DevMode {
		return &RedisStorage{cfg: cfg, pool: nil}, nil
	}

	pool := NewPool(cfg)
	_, err := pool.Dial()
	if err != nil {
		return nil, errors.Wrap(err, "invalid redis configuration url")
	}

	return &RedisStorage{
		cfg:   cfg,
		pool:  pool,
		stats: stats,
	}, nil
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

func (s *RedisStorage) GetByKey(bucket string, key []byte) ([]byte, error) {
	conn := s.pool.Get()
	defer conn.Close()

	rawSensitive, err := redis.Bytes(conn.Do(commandGet, key))
	if err != nil && err.Error() == redis.ErrNil.Error() {
		return nil, err
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to perform %s command, data wasn't retrieved", commandGet)
	}

	return rawSensitive, nil
}

// Save saves any value as json object with provided key
func (s *RedisStorage) Save(bucket string, key, value []byte, ttl int64) error {
	if s.cfg.DevMode {
		return nil
	}

	conn := s.pool.Get()
	defer conn.Close()

	sensitiveMarshalled, err := json.Marshal(value)
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}

	_, err = conn.Do(commandSet, s.stats.CurrentLastKey, sensitiveMarshalled, commandExpire, ttl)
	if err != nil {
		return errors.Wrapf(err, "failed to perform %s command, data wasn't saved", commandSet)
	}
	s.stats.UpdateKey()

	return nil
}

func (s *RedisStorage) getAllKeys() ([]string, error) {
	conn := s.pool.Get()
	defer conn.Close()

	keys, err := redis.Strings(conn.Do("KEYS", "*"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all data")
	}
	return keys, nil
}

func (s *RedisStorage) GetBroadcast() ([]models.Message, error) {
	keys, err := s.getAllKeys()
	if err != nil {
		return nil, err
	}

	data := make([]models.Message, 0)
	for _, key := range keys {
		rawMsg, err := s.GetByKey("", []byte(key))
		if err != nil {
			return nil, errors.Wrap(err, "failed to get data by key")
		}

		var msg models.Message
		err = json.Unmarshal(rawMsg, &models.Message{})
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal data")
		}

		data = append(data, msg)
	}
	return data, nil
}

func (s *RedisStorage) GetDirect(bucket string) ([]models.Message, error) {
	return nil, nil
}
