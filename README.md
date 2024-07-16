# recache
redis cache for database

# example
```go
type s struct {
	ID int64  `json:"id" gorm:"primaryKey"`
	A  int    `json:"a"`
	B  string `json:"b"`
}

func main() {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	db.AutoMigrate(&s{})
	redisServer, _ := miniredis.Run()
	red := redis.NewClient(&redis.Options{
		Addr: redisServer.Addr(),
	})
	cache := recache.NewCache(red, recache.WithStatInterval(time.Second))
	cache.Exec(context.Background(), func() error {
		return db.Create(&s{
			ID: 1,
			A:  100,
			B:  "b",
		}).Error
	}, "key1")
	get(cache, db, 1)
	get(cache, db, 1)
	select {}
}

func get(cache *recache.Cache, db *gorm.DB, id int64) {
	var model s
	cache.Query(context.Background(), "key1", &model, func(v any) (bool, error) {
		err := db.Model(&s{}).Where("id = ?", id).First(&v).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	fmt.Printf("%v\n", &model)
}
```

* `recache.NewCache` returns a `Cache` instance, optionally privided with options
* `Cache.Exec` do the write operation and then delete the redis data
* `Cache.Query` first search for data from redis, if not found, execute the incoming db query parameters. The db query function needs to return whether it is found
* `Cache.Get` query only from redis
* `Cache.Set` set only by redis
* `Cache.Del` delete from redis
* `Cache.SetWithExpire` set by redis with expiration

### Option
* `WithLogger` customize logger instance
* `WithExpire` customize the expiration time of redis data. The expiration time is really less than or equal to this value.
* `WithHystrixConfig` customize `Hystrix` configration
* `WithMetricsPort` open `Hystrix` metrics  on configuration port
* `WithStatInterval` customize stat infomation interval