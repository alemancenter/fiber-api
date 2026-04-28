package services

import (
	"runtime"
	"time"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/internal/repositories"
)

type HealthService interface {
	GetHealthStatus() (map[string]interface{}, int, bool)
}

type healthService struct {
	repo      repositories.HealthRepository
	startTime time.Time
}

func NewHealthService(repo repositories.HealthRepository) HealthService {
	return &healthService{repo: repo, startTime: time.Now()}
}

func (s *healthService) GetHealthStatus() (map[string]interface{}, int, bool) {
	cfg := config.Get()
	dbHealth := s.repo.CheckDatabases()
	redisHealth := s.repo.CheckRedis()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	allHealthy := true
	for _, ok := range dbHealth {
		if !ok {
			allHealthy = false
			break
		}
	}
	for _, ok := range redisHealth {
		if !ok {
			allHealthy = false
			break
		}
	}

	status := "healthy"
	httpStatus := 200 // fiber.StatusOK
	if !allHealthy {
		status = "degraded"
		httpStatus = 503 // fiber.StatusServiceUnavailable
	}

	return map[string]interface{}{
		"status":    status,
		"app_name":  cfg.App.Name,
		"env":       cfg.App.Env,
		"uptime":    time.Since(s.startTime).String(),
		"databases": dbHealth,
		"redis":     redisHealth,
		"memory": map[string]interface{}{
			"alloc_mb":   memStats.Alloc / 1024 / 1024,
			"sys_mb":     memStats.Sys / 1024 / 1024,
			"num_gc":     memStats.NumGC,
			"goroutines": runtime.NumGoroutine(),
		},
		"timestamp": time.Now().UTC(),
	}, httpStatus, allHealthy
}