package recache

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	notfoundPlaceHolder = "*"
)

type cacheConfig struct {
	expire time.Duration
}

type cache struct {
	cli  redis.Cmdable
	log  logger
	stat *stat
	cfg  *cacheConfig
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
		return redis.Nil
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
	bs, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return c.cli.Set(ctx, key, string(bs), randExpire(c.cfg.expire)).Err()
}

func randExpire(d time.Duration) time.Duration {
	rand.Seed(time.Now().UnixNano())
	sec := math.Ceil(rand.Float64() * float64(d.Seconds()))
	return time.Second * time.Duration(sec)
}

func NewCache() *cache {
	return &cache{}
}
