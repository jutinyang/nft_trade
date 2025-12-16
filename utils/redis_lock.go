package utils

import (
	"context"
	"errors"
	"fmt"
	"time"

	// 为原生Redis客户端添加别名，解决命名冲突
	goredis "github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"

	// 为redsync的redis接口包添加别名，避免冲突
	goredisadapter "github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/google/uuid"
)

// -------------------------- 全局变量（导出需首字母大写） --------------------------
// RedisClient 全局Redis客户端（导出，供外部包直接使用）
var RedisClient *goredis.Client

// Redisync 全局RedSync实例（用于RedLock分布式锁）
var Redisync *redsync.Redsync

// RedisLockInst 全局RedisLock实例（导出，供外部包调用其方法）
var RedisLockInst *RedisLock

// -------------------------- RedisLock结构体（封装基础锁逻辑） --------------------------
// RedisLock 封装Redis分布式锁的基础操作（SetNX+Lua脚本）
type RedisLock struct {
	client *goredis.Client // 复用全局Redis客户端，避免重复创建
	ctx    context.Context // 支持外部上下文传递
}

// -------------------------- 初始化函数（核心入口） --------------------------
// InitRedis 初始化Redis客户端、RedSync、RedisLock实例（需在程序启动时调用）
// 参数：addr(Redis地址)、password(Redis密码)、db(Redis数据库编号)
// 返回：错误信息（初始化失败时）
func InitRedis(addr, password string, db int) error {
	// 1. 初始化全局Redis客户端
	RedisClient = goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
		// 可选：配置连接池（生产环境建议添加）
		PoolSize: 10,
	})

	// 校验Redis连接可用性
	if err := RedisClient.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	// 2. 初始化RedSync（支持RedLock分布式锁）
	adapterPool := goredisadapter.NewPool(RedisClient)
	Redisync = redsync.New(adapterPool)

	// 3. 初始化全局RedisLock实例（复用全局Redis客户端）
	RedisLockInst = &RedisLock{
		client: RedisClient,
		ctx:    context.Background(), // 默认为后台上下文，可通过SetCtx修改
	}

	return nil
}

// -------------------------- RedisLock实例方法 --------------------------
// SetCtx 手动设置上下文（支持外部传递超时/取消上下文）
func (rl *RedisLock) SetCtx(ctx context.Context) {
	rl.ctx = ctx
}

// Client 返回Redis客户端（供外部包调用，兼容旧逻辑）
func (rl *RedisLock) Client() *goredis.Client {
	return rl.client
}

// Lock 基础分布式锁：加锁（存入唯一标识，SetNX+过期时间）
// 参数：key(锁键)、expire(锁过期时间)
// 返回：lockID(锁唯一标识)、error(加锁失败原因)
func (rl *RedisLock) Lock(key string, expire time.Duration) (string, error) {
	if rl.client == nil {
		return "", errors.New("redis client not initialized")
	}

	// 生成唯一锁标识（防止误删其他客户端的锁）
	lockID := uuid.NewString()
	// SetNX：不存在则设置，原子操作（同时指定过期时间，避免死锁）
	res, err := rl.client.SetNX(rl.ctx, key, lockID, expire).Result()
	if err != nil {
		return "", fmt.Errorf("setnx failed: %w", err)
	}
	if !res {
		return "", errors.New("key is locked by other client")
	}

	return lockID, nil
}

// Unlock 基础分布式锁：解锁（Lua脚本原子校验+删除）
// 参数：key(锁键)、lockID(加锁时的唯一标识)
// 返回：error(解锁失败原因，包括锁不匹配、锁已过期)
func (rl *RedisLock) Unlock(key, lockID string) error {
	if rl.client == nil {
		return errors.New("redis client not initialized")
	}

	// Lua脚本：原子校验锁标识并删除（避免并发误删）
	luaScript := `
		if redis.call('get', KEYS[1]) == ARGV[1] then
			return redis.call('del', KEYS[1])
		else
			return 0
		end
	`

	// 执行Lua脚本
	res, err := rl.client.Eval(rl.ctx, luaScript, []string{key}, lockID).Result()
	if err != nil {
		return fmt.Errorf("eval lua script failed: %w", err)
	}

	// 脚本返回0表示锁标识不匹配或锁已过期
	if res.(int64) == 0 {
		return errors.New("lock ID not match or key has expired")
	}

	return nil
}

// -------------------------- RedSync分布式锁（高级锁，支持RedLock） --------------------------
// GetRedisLock 获取RedSync分布式锁（支持多Redis节点的RedLock算法）
// 参数：ctx(上下文)、key(锁键)、expire(锁过期时间)
// 返回：mutex(锁实例)、error(加锁失败原因)
func GetRedisLock(ctx context.Context, key string, expire time.Duration) (*redsync.Mutex, error) {
	if Redisync == nil {
		return nil, errors.New("redsync not initialized")
	}

	// 创建Mutex实例（指定过期时间）
	mutex := Redisync.NewMutex(key, redsync.WithExpiry(expire))
	// 加锁（支持上下文）
	if err := mutex.LockContext(ctx); err != nil {
		return nil, fmt.Errorf("redsync lock failed: %w", err)
	}

	return mutex, nil
}

// ReleaseRedisLock 释放RedSync分布式锁
// 参数：mutex(锁实例)
// 返回：error(解锁失败原因，包括锁已过期)
func ReleaseRedisLock(mutex *redsync.Mutex) error {
	if mutex == nil {
		return errors.New("mutex is nil")
	}

	// Unlock返回：bool(是否解锁成功)、error(执行错误)
	ok, err := mutex.Unlock()
	if err != nil {
		return fmt.Errorf("redsync unlock failed: %w", err)
	}
	if !ok {
		return errors.New("mutex has expired or not held")
	}

	return nil
}

// -------------------------- 兼容旧的NewRedisLock函数（可选，逐步淘汰） --------------------------
// NewRedisLock 兼容旧逻辑的RedisLock创建函数（建议使用全局RedisLockInst，避免重复创建客户端）
// 参数：addr(Redis地址)、password(Redis密码)、db(Redis数据库编号)
// 返回：RedisLock实例、error(创建失败原因)
func NewRedisLock(addr, password string, db int) (*RedisLock, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis connect failed: %w", err)
	}

	return &RedisLock{
		client: client,
		ctx:    context.Background(),
	}, nil
}
