package sessions

import (
	"encoding/json"
	"log"
	"math"
	"time"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
	"gitlab.inn4science.com/ctp/hermes/config"
)

const boltDBBucket = "sessions"

type BoltDBStorage struct {
	cfg  config.BoltDBConfig
	conn *bolt.DB
}

func NewBoltDBStorage(cfg config.BoltDBConfig) (*BoltDBStorage, error) {
	options := &bolt.Options{
		// Timeout is the amount of time to wait to obtain a file lock
		Timeout: time.Duration(cfg.Timeout * int64(math.Pow10(9))),
		// Open database in read-only mode
		ReadOnly: cfg.ReadOnly,
		// InitialMmapSize is the initial mmap size of the database in bytes
		InitialMmapSize: 0,
	}
	conn, err := bolt.Open(cfg.FilePath, 0600, options)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to initialize the bolt store")
	}

	return &BoltDBStorage{
		conn: conn,
		cfg:  cfg,
	}, nil
}

func (b *BoltDBStorage) CheckConn() error {
	conn, err := bolt.Open(b.cfg.FilePath, 0666, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func (b *BoltDBStorage) CloseConnection() error {
	return b.conn.Close()
}

func (b *BoltDBStorage) GetSession(key string) (*Session, error) {
	session := new(Session)

	err := b.conn.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(boltDBBucket))
		v := b.Get([]byte(key))

		err := json.Unmarshal(v, session)
		if err != nil {
			return errors.Wrap(err, "failed to unmarshal session from from boltdb")
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to view the data by key")
	}
	return session, err
}

func (b *BoltDBStorage) SaveAsJSON(key string, value interface{}, _ int64) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}

	return b.conn.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(boltDBBucket))
		if b == nil {
			b, err = tx.CreateBucket([]byte(boltDBBucket))
			if err != nil {
				return err
			}
			log.Printf("boltdb saved item with key %s value %s", key, raw)
			return b.Put([]byte(key), raw)
		}

		log.Printf("boltdb saved item with key %s value %s", key, raw)
		return b.Put([]byte(key), raw)
	})
}
