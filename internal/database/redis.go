package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RedisManager struct {
	default_ *redis.Client
	cache    *redis.Client
	queue    *redis.Client
	prefix   string
}

var (
	redisManager     *RedisManager
	redisManagerOnce sync.Once
)

// GetRedis returns the singleton Redis manager
func GetRedis() *RedisManager {
	redisManagerOnce.Do(func() {
		cfg := config.Get().Redis
		redisManager = &RedisManager{
			prefix:   cfg.Prefix,
			default_: newRedisClient(cfg, cfg.DB),
			cache:    newRedisClient(cfg, cfg.CacheDB),
			queue:    newRedisClient(cfg, cfg.QueueDB),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := redisManager.default_.Ping(ctx).Err(); err != nil {
			logger.Fatal("failed to connect to Redis", zap.Error(err))
		}
		logger.Info("Redis connected", zap.String("addr", cfg.Addr()))
	})
	return redisManager
}

func newRedisClient(cfg config.RedisConfig, db int) *redis.Client {
	opts := &redis.Options{
		Addr:         cfg.Addr(),
		Password:     cfg.Password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
		MinIdleConns: 5,
		MaxRetries:   3,
	}
	return redis.NewClient(opts)
}

// Default returns the default Redis client
func (r *RedisManager) Default() *redis.Client { return r.default_ }

// Cache returns the cache Redis client
func (r *RedisManager) Cache() *redis.Client { return r.cache }

// Queue returns the queue Redis client
func (r *RedisManager) Queue() *redis.Client { return r.queue }

// Key builds a namespaced cache key
func (r *RedisManager) Key(parts ...string) string {
	key := r.prefix
	for _, part := range parts {
		key += ":" + part
	}
	return key
}

// CountryKey builds a country-specific cache key
func (r *RedisManager) CountryKey(country string, parts ...string) string {
	base := r.Key(country)
	for _, part := range parts {
		base += ":" + part
	}
	return base
}

// Set stores a value in cache
func (r *RedisManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.cache.Set(ctx, key, value, ttl).Err()
}

// Get retrieves a value from cache
func (r *RedisManager) Get(ctx context.Context, key string) (string, error) {
	return r.cache.Get(ctx, key).Result()
}

// Del deletes a key from cache
func (r *RedisManager) Del(ctx context.Context, keys ...string) error {
	return r.cache.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func (r *RedisManager) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.cache.Exists(ctx, key).Result()
	return n > 0, err
}

// IncrBy increments a key's value
func (r *RedisManager) IncrBy(ctx context.Context, key string, val int64) (int64, error) {
	return r.cache.IncrBy(ctx, key, val).Result()
}

// Expire sets expiry on a key
func (r *RedisManager) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.cache.Expire(ctx, key, ttl).Err()
}

// GetTTL returns remaining TTL
func (r *RedisManager) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return r.cache.TTL(ctx, key).Result()
}

// FlushCountryCache flushes all cache keys for a given country
func (r *RedisManager) FlushCountryCache(ctx context.Context, country string) error {
	pattern := fmt.Sprintf("%s:%s:*", r.prefix, country)
	return r.flushPattern(ctx, pattern)
}

// FlushPattern deletes all keys matching a pattern
func (r *RedisManager) flushPattern(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, nextCursor, err := r.cache.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := r.cache.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// HealthCheck checks Redis connectivity
func (r *RedisManager) HealthCheck() map[string]bool {
	ctx := context.Background()
	return map[string]bool{
		"default": r.default_.Ping(ctx).Err() == nil,
		"cache":   r.cache.Ping(ctx).Err() == nil,
		"queue":   r.queue.Ping(ctx).Err() == nil,
	}
}

// GetInfo returns Redis server information
func (r *RedisManager) GetInfo(ctx context.Context) (string, error) {
	return r.default_.Info(ctx).Result()
}

// SetNX sets key only if it doesn't exist (for distributed locks)
func (r *RedisManager) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	return r.default_.SetNX(ctx, key, value, ttl).Result()
}

// ListKeys returns one bounded page of keys matching a pattern from the cache DB.
func (r *RedisManager) ListKeys(ctx context.Context, pattern string, limit, offset int) ([]string, bool, error) {
	if pattern == "" {
		pattern = "*"
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	result := make([]string, 0, limit)
	var cursor uint64
	skipped := 0
	scanCount := int64(limit)
	if scanCount < 100 {
		scanCount = 100
	}

	for {
		keys, nextCursor, err := r.cache.Scan(ctx, cursor, pattern, scanCount).Result()
		if err != nil {
			return nil, false, err
		}
		for _, key := range keys {
			if skipped < offset {
				skipped++
				continue
			}
			if len(result) < limit {
				result = append(result, key)
				continue
			}
			return result, true, nil
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return result, false, nil
}

// Redis is a convenience function to get the Redis manager
func Redis() *RedisManager {
	return GetRedis()
}
