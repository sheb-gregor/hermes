package sessions

import (
	"log"

	"gitlab.inn4science.com/ctp/hermes/config"
)

type Storage interface {
	CheckConn() error
	CloseConnection() error
	GetSession(key string) (*Session, error)
	SaveAsJSON(key string, value interface{}, ttl int64) error
}

func NewStorage(cfg config.Cfg) Storage {
	switch cfg.CacheStorageType {
	case config.BoltDBStorageType:
		boltdb, err := NewBoltDBStorage(cfg.BoltDB)
		if err != nil {
			log.Fatalf("boltdb err: %s", err)
		}
		return boltdb
	default:
		redis, err := NewRedisStorage(cfg.Redis)
		if err != nil {
			log.Fatalf("redis err: %s", err)
		}
		return redis
	}
}
