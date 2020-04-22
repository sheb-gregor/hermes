package cache

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/models"
)

const (
	BroadcastBucket = "broadcast"
)

type Storage interface {
	CheckConn() error
	CloseConnection() error
	GetByKey(bucket string, key []byte) ([]byte, error)
	Save(bucket string, key, value []byte, ttl int64) error

	GetBroadcast() ([]models.Message, error)
	GetDirect(bucket string) ([]models.Message, error)
}

func NewStorage(cfg config.CacheCfg) (Storage, error) {
	if cfg.Disable {
		return new(storageStub), nil
	}

	stats := NewBucketStats()

	switch cfg.Type {
	case config.StorageTypeNutsDB:
		nutsdb, err := NewNutsDBStorage(cfg.NutsDB, stats)
		if err != nil {
			return nil, errors.Wrap(err, "nutsdb init storage err")
		}
		return nutsdb, nil
	default:
		redis, err := NewRedisStorage(cfg.Redis, stats)
		if err != nil {
			return nil, errors.Wrap(err, "redis init storage err")
		}
		return redis, nil
	}
}

type BucketStats struct {
	CurrentInitialKey string
	CurrentLastKey    string
}

func NewBucketStats() BucketStats {
	key := fmt.Sprintf("event_%d", time.Now().UTC().UnixNano())
	return BucketStats{
		CurrentInitialKey: key,
		CurrentLastKey:    key,
	}
}

func (s *BucketStats) UpdateKey() {
	s.CurrentLastKey = fmt.Sprintf("event_%d", time.Now().UTC().UnixNano())
}

type storageStub struct{}

func (s *storageStub) CheckConn() error { return nil }

func (s *storageStub) CloseConnection() error { return nil }

func (s *storageStub) GetByKey(string, []byte) ([]byte, error) { return nil, nil }

func (s *storageStub) Save(string, []byte, []byte, int64) error { return nil }

func (s *storageStub) GetBroadcast() ([]models.Message, error) { return nil, nil }

func (s *storageStub) GetDirect(string) ([]models.Message, error) { return nil, nil }
