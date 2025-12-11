package utils

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	goredis "github.com/go-redsync/redsync/v4/redis/goredis/v8"
)

var RedisClient *redis.Client
var Redisync *redsync.Redsync

func InitRedis(addr, password string, db int) {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// 使用正确的适配器
	pool := goredis.NewPool(RedisClient)
	// 注意：redsync.New 需要 []redsync.Pool
	Redisync = redsync.New(pool) // 注意：redsync.New 直接接受 pool

}

// GetRedisLock 获取分布式锁
func GetRedisLock(ctx context.Context, key string, expire time.Duration) (*redsync.Mutex, error) {
	// 注意：这里应该使用 Redisync 而不是 Redisync
	mutex := Redisync.NewMutex(key, redsync.WithExpiry(expire))
	if err := mutex.LockContext(ctx); err != nil {
		return nil, err
	}
	return mutex, nil
}

// ReleaseRedisLock 释放分布式锁
func ReleaseRedisLock(mutex *redsync.Mutex) error {
	_, err := mutex.Unlock()
	return err
}
