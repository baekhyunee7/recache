package recache

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

var (
	notfoundPlaceHolder = "*"
	placeHolderError    = errors.New("placeHolder")
	NotFoundError       = errors.New("not found")
)

type cacheConfig struct {
	expire time.Duration
}

type cache struct {
	cli  redis.Cmdable
	log  logger
	stat *stat
	cfg  *cacheConfig
	sf   *singleflight.Group
}

func (c *cache) Get(ctx context.Context, key string, v any) error {
	c.stat.incrementTotal()
	result, err := c.cli.Get(ctx, key).Result()
	if err != nil {
		c.stat.incrementMiss()
		return err
	}
	err = json.Unmarshal([]byte(result), v)
	if err != nil {
		c.log.Errorf("unmarshal data fail, key: %s, value: %s, err: %v", key, result, err)
		if err = c.Del(ctx, key); err != nil {
			c.log.Errorf("del invalid key fail, key: %s", key)
		}
		// enable load db
		return redis.Nil
	}
	c.stat.incrementHit()
	if result == notfoundPlaceHolder {
		return placeHolderError
	}
	return nil
}

func (c *cache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	err := c.cli.Del(ctx, keys...).Err()
	if err != nil {
		c.log.Errorf("cache del fail, keys: %#v, err: %v", keys, err)
		return err
	}
	return nil
}

func (c *cache) Set(ctx context.Context, key string, val any) error {
	return c.SetWithExpire(ctx, key, val, randExpire(c.cfg.expire))
}

func randExpire(d time.Duration) time.Duration {
	rand.Seed(time.Now().UnixNano())
	sec := math.Ceil(rand.Float64() * float64(d.Seconds()))
	return time.Second * time.Duration(sec)
}

func (c *cache) SetWithExpire(ctx context.Context, key string, val any, expire time.Duration) error {
	bs, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return c.cli.Set(ctx, key, string(bs), expire).Err()
}

func (c *cache) Query(ctx context.Context, key string, val any, query func(v any) (bool, error)) error {
	val, err, shared := c.sf.Do(key, func() (any, error) {
		err := c.Get(ctx, key, val)
		if err != nil {
			if err == redis.Nil {
				found, err := query(val)
				if err != nil {
					c.stat.incrementDbFails()
					return nil, err
				}
				if !found {
					err = c.setPlaceHolder(ctx, key)
					c.log.Warnf("set placeholder fail, key: %s, err: %v", key, err)
					return nil, NotFoundError
				}
				return val, nil
			} else if err == placeHolderError {
				return nil, NotFoundError
			}
			return nil, err
		}
		// found in cache
		return val, nil
	})
	if err != nil && err != NotFoundError {
		return err
	}
	if shared {
		c.stat.incrementTotal()
		c.stat.incrementShared()
	}
	return err
}

func (c *cache) setPlaceHolder(ctx context.Context, key string) error {
	return c.cli.Set(ctx, key, notfoundPlaceHolder, randExpire(c.cfg.expire)).Err()
}

func (c *cache) Exec(ctx context.Context, dbFunc func() error, keys ...string) error {
	err := dbFunc()
	if err != nil {
		return err
	}
	return c.Del(ctx, keys...)
}

func NewCache() *cache {
	return &cache{}
}
