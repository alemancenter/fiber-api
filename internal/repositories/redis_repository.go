package repositories

import (
	"context"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
)

type RedisRepository interface {
	ListKeys(ctx context.Context, pattern string) ([]string, error)
	SetKey(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	DeleteKey(ctx context.Context, key string) error
	HealthCheck() map[string]bool
	GetInfo(ctx context.Context) (string, error)
}

type redisRepository struct{}

func NewRedisRepository() RedisRepository {
	return &redisRepository{}
}

func (r *redisRepository) ListKeys(ctx context.Context, pattern string) ([]string, error) {
	return database.Redis().ListKeys(ctx, pattern)
}

func (r *redisRepository) SetKey(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return database.Redis().Cache().Set(ctx, key, value, ttl).Err()
}

func (r *redisRepository) DeleteKey(ctx context.Context, key string) error {
	return database.Redis().Cache().Del(ctx, key).Err()
}

func (r *redisRepository) HealthCheck() map[string]bool {
	return database.Redis().HealthCheck()
}

func (r *redisRepository) GetInfo(ctx context.Context) (string, error) {
	return database.Redis().GetInfo(ctx)
}
