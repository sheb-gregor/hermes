package cache

import (
	"encoding/json"

	"hermes/config"
	"hermes/models"

	"github.com/pkg/errors"
	"github.com/xujiajun/nutsdb"
)

type NutsDBStorage struct {
	cfg  config.NutsDBCfg
	conn *nutsdb.DB

	stats BucketStats
}

func NewNutsDBStorage(cfg config.NutsDBCfg, stats BucketStats) (*NutsDBStorage, error) {
	options := nutsdb.Options{
		Dir:                  cfg.Path,
		SegmentSize:          cfg.SegmentSize,
		SyncEnable:           true,
		StartFileLoadingMode: 0,
	}
	conn, err := nutsdb.Open(options)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to initialize the nutsdb store")
	}

	return &NutsDBStorage{
		conn:  conn,
		cfg:   cfg,
		stats: stats,
	}, nil
}

func (b *NutsDBStorage) CheckConn() error {
	conn, err := nutsdb.Open(nutsdb.Options{Dir: b.cfg.Path})
	if err != nil {
		return err
	}

	return conn.Close()
}

func (b *NutsDBStorage) CloseConnection() error {
	return b.conn.Close()
}

func (b *NutsDBStorage) GetByKey(bucket string, key []byte) ([]byte, error) {
	var err error
	var valueEntry *nutsdb.Entry

	err = b.conn.View(func(tx *nutsdb.Tx) error {
		valueEntry, err = tx.Get(bucket, key)
		if err != nil {
			return errors.Wrap(err, "failed to get the data by key")
		}

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to view the data by key")
	}
	return valueEntry.Value, err
}

func (b *NutsDBStorage) Save(bucket string, key, value []byte, ttl int64) error {
	var nutsKey []byte
	if key == nil {
		nutsKey = []byte(b.stats.CurrentLastKey)
	} else {
		nutsKey = key
	}

	return b.conn.Update(func(tx *nutsdb.Tx) error {
		err := tx.Put(bucket, nutsKey, value, uint32(ttl))
		if err != nil {
			return err
		}

		b.stats.UpdateKey()
		return nil
	})
}

func (b *NutsDBStorage) getData(bucket string) ([]models.Message, error) {
	var data []models.Message

	err := b.conn.View(func(tx *nutsdb.Tx) error {
		entries, err := tx.GetAll(bucket)
		if err != nil {
			return errors.Wrap(err, "failed to get all data from nutsdb")
		}

		for _, entry := range entries {
			var raw models.Message
			err = json.Unmarshal(entry.Value, &raw)
			if err != nil {
				return errors.Wrap(err, "failed to unmarshal data from nutsdb")
			}
			data = append(data, raw)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all data from nutsdb")
	}

	return data, nil
}

func (b *NutsDBStorage) GetBroadcast() ([]models.Message, error) {
	return b.getData(BroadcastBucket)
}

func (b *NutsDBStorage) GetDirect(bucket string) ([]models.Message, error) {
	return b.getData(bucket)
}
