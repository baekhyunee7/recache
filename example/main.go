package main

import (
	"context"
	"fmt"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/baekhyunee7/recache"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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
