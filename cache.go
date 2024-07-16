package recache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/afex/hystrix-go/hystrix"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

var (
	notfoundPlaceHolder = "*"
	errPlaceHolder      = errors.New("placeHolder")
	ErrNotFound         = errors.New("not found")
)

type cacheConfig struct {
	expire       time.Duration
	log          logger
	metricConfig *metricConfig
	statInterval time.Duration
}

type metricConfig struct {
	port int
}

type Cache struct {
	cli  redis.Cmdable
	stat *stat
	cfg  *cacheConfig
	sf   *singleflight.Group
}

func (c *Cache) Get(ctx context.Context, key string, v any) error {
	c.stat.incrementTotal()
	var result string
	err := hystrix.DoC(ctx, fmt.Sprintf("Get: %s", key), func(ctx context.Context) error {
		res, err := c.cli.Get(ctx, key).Result()
		if err != nil {
			return err
		}
		result = res
		return nil
	}, nil)
	if err != nil {
		c.stat.incrementMiss()
		return err
	}
	err = json.Unmarshal([]byte(result), v)
	if err != nil {
		c.cfg.log.Errorf("unmarshal data fail, key: %s, value: %s, err: %v", key, result, err)
		if err = c.Del(ctx, key); err != nil {
			c.cfg.log.Errorf("del invalid key fail, key: %s", key)
		}
		// enable load db
		return redis.Nil
	}
	c.stat.incrementHit()
	if result == notfoundPlaceHolder {
		return errPlaceHolder
	}
	return nil
}

func (c *Cache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	err := hystrix.DoC(ctx, fmt.Sprintf("Del: %#v", keys), func(ctx context.Context) error {
		return c.cli.Del(ctx, keys...).Err()
	}, nil)
	if err != nil {
		c.cfg.log.Errorf("cache del fail, keys: %#v, err: %v", keys, err)
		return err
	}
	return nil
}

func (c *Cache) Set(ctx context.Context, key string, val any) error {
	return c.SetWithExpire(ctx, key, val, randExpire(c.cfg.expire))
}

func randExpire(d time.Duration) time.Duration {
	rand.Seed(time.Now().UnixNano())
	sec := math.Ceil(rand.Float64() * float64(d.Seconds()))
	return time.Second * time.Duration(sec)
}

func (c *Cache) SetWithExpire(ctx context.Context, key string, val any, expire time.Duration) error {
	bs, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return hystrix.DoC(ctx, fmt.Sprintf("Set: %s", key), func(ctx context.Context) error {
		return c.cli.Set(ctx, key, string(bs), expire).Err()
	}, nil)
}

func (c *Cache) Query(ctx context.Context, key string, val any, query func(v any) (bool, error)) error {
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
					c.cfg.log.Warnf("set placeholder fail, key: %s, err: %v", key, err)
					return nil, ErrNotFound
				}
				err = c.Set(ctx, key, val)
				if err != nil {
					c.cfg.log.Warnf("cache after query fail, key: %s, err: %v", key, err)
				}
				return val, nil
			} else if err == errPlaceHolder {
				return nil, ErrNotFound
			}
			return nil, err
		}
		// found in cache
		return val, nil
	})
	if err != nil && err != ErrNotFound {
		return err
	}
	if shared {
		c.stat.incrementTotal()
		c.stat.incrementShared()
	}
	return err
}

func (c *Cache) setPlaceHolder(ctx context.Context, key string) error {
	return hystrix.DoC(ctx, fmt.Sprintf("Set-Ph: %s", key), func(ctx context.Context) error {
		return c.cli.Set(ctx, key, notfoundPlaceHolder, randExpire(c.cfg.expire)).Err()
	}, nil)
}

func (c *Cache) Exec(ctx context.Context, dbFunc func() error, keys ...string) error {
	err := dbFunc()
	if err != nil {
		return err
	}
	return c.Del(ctx, keys...)
}

func NewCache(cli redis.Cmdable, opts ...option) *Cache {
	cfg := &cacheConfig{
		expire:       time.Minute,
		log:          &defaultLogger{},
		statInterval: time.Minute,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.metricConfig != nil {
		hystrixStreamHandler := hystrix.NewStreamHandler()
		hystrixStreamHandler.Start()
		go http.ListenAndServe(fmt.Sprintf("0.0.0.0: %d", cfg.metricConfig.port), hystrixStreamHandler)
	}
	return &Cache{
		cli:  cli,
		stat: NewStat(cfg.log, cfg.statInterval),
		cfg:  cfg,
		sf:   &singleflight.Group{},
	}
}

type option func(*cacheConfig)

func WithLogger(log logger) option {
	return func(cc *cacheConfig) {
		cc.log = log
	}
}

func WithExpire(d time.Duration) option {
	return func(cc *cacheConfig) {
		cc.expire = d
	}
}

func WithHystrixConfig(cfg map[string]hystrix.CommandConfig) option {
	return func(_ *cacheConfig) {
		hystrix.Configure(cfg)
	}
}

func WithMetricsPort(port int) option {
	return func(cc *cacheConfig) {
		cc.metricConfig = &metricConfig{
			port: port,
		}
	}
}

func WithStatInterval(d time.Duration) option {
	return func(cc *cacheConfig) {
		cc.statInterval = d
	}
}
