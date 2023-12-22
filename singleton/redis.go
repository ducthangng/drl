package singleton

import "github.com/go-redis/redis/v8"

var RedisClient *redis.Client

func set() {
	RedisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
}

func GetRedisClient() *redis.Client {
	if RedisClient == nil {
		set()
	}

	return RedisClient
}
