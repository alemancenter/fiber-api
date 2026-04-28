package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
)

// GetOrSet tries the Redis cache first. On a miss it calls fetch(), caches the
// result as JSON, and returns it. T must be JSON-serializable.
func GetOrSet[T any](ctx context.Context, key string, ttl time.Duration, fetch func() (T, error)) (T, error) {
	rdb := database.Redis()

	if cached, err := rdb.Get(ctx, key); err == nil {
		var result T
		if json.Unmarshal([]byte(cached), &result) == nil {
			return result, nil
		}
	}

	result, err := fetch()
	if err != nil {
		return result, err
	}

	if data, err := json.Marshal(result); err == nil {
		_ = rdb.Set(ctx, key, data, ttl)
	}
	return result, nil
}

// InvalidateCache deletes one or more cache keys immediately.
func InvalidateCache(keys ...string) {
	if len(keys) == 0 {
		return
	}
	database.Redis().Del(context.Background(), keys...)
}
