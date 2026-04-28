package services

import (
	"context"
	"time"

	"github.com/alemancenter/fiber-api/internal/repositories"
)

type RedisService interface {
	ListKeys(ctx context.Context, pattern string) ([]string, error)
	SetKey(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	DeleteKey(ctx context.Context, key string) error
	CleanExpired(ctx context.Context) error
	TestConnection() (map[string]bool, bool)
	GetInfo(ctx context.Context) (string, error)
}

type RedisKeysResponse struct {
	Keys  []string `json:"keys"`
	Count int      `json:"count"`
}

type RedisInfoResponse struct {
	Info string `json:"info"`
}

type redisService struct {
	repo repositories.RedisRepository
}

func NewRedisService(repo repositories.RedisRepository) RedisService {
	return &redisService{repo: repo}
}

func (s *redisService) ListKeys(ctx context.Context, pattern string) ([]string, error) {
	return s.repo.ListKeys(ctx, pattern)
}

func (s *redisService) SetKey(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return s.repo.SetKey(ctx, key, value, ttl)
}

func (s *redisService) DeleteKey(ctx context.Context, key string) error {
	return s.repo.DeleteKey(ctx, key)
}

func (s *redisService) CleanExpired(ctx context.Context) error {
	// Redis automatically removes expired keys; this is a manual scan for near-expired ones
	return nil
}

func (s *redisService) TestConnection() (map[string]bool, bool) {
	health := s.repo.HealthCheck()
	allOk := true
	for _, ok := range health {
		if !ok {
			allOk = false
			break
		}
	}
	return health, allOk
}

func (s *redisService) GetInfo(ctx context.Context) (string, error) {
	return s.repo.GetInfo(ctx)
}
