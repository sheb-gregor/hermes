package cache

import (
	"time"

	"hermes/config"

	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
)

func NewPool(conf config.RedisConf) *redis.Pool {
	return &redis.Pool{
		Dial:            dial(conf.URL()),
		TestOnBorrow:    ping(time.Duration(conf.PingInterval) * time.Second),
		MaxIdle:         conf.MaxIdleConn,
		MaxActive:       conf.MaxActiveConn,
		IdleTimeout:     time.Duration(conf.IdleTimeout) * time.Second,
		Wait:            false,
		MaxConnLifetime: 0,
	}
}

func dial(url string) func() (redis.Conn, error) {
	return func() (redis.Conn, error) {
		c, err := redis.DialURL(url)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to dial")
		}
		return c, err
	}

}

func ping(pingInterval time.Duration) func(c redis.Conn, t time.Time) error {
	return func(c redis.Conn, t time.Time) error {
		if time.Since(t) < pingInterval {
			return nil
		}

		_, err := c.Do("PING")
		if err != nil {
			return errors.Wrap(err, "ping command failed")
		}

		return nil
	}
}
