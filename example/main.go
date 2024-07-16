package main

import (
	"context"
	"fmt"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/baekhyunee7/recache"
	"github.com/redis/go-redis/v9"
)

type s struct {
	A int    `json:"a"`
	B string `json:"b"`
}

func main() {
	redisServer, _ := miniredis.Run()
	red := redis.NewClient(&redis.Options{
		Addr: redisServer.Addr(),
	})
	cache := recache.NewCache(red, recache.WithStatInterval(time.Second))
	cache.Set(context.Background(), "key1", &s{
		A: 100,
		B: "b",
	})
	get(cache)
	cache.Del(context.Background(), "key1")
	get(cache)
	select {}
}

func get(cache *recache.Cache) {
	var s s
	cache.Get(context.Background(), "key1", &s)
	fmt.Printf("%v\n", &s)
}
