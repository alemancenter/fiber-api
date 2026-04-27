package health

import (
	"runtime"
	"time"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/gofiber/fiber/v2"
)

var startTime = time.Now()

// Handler contains health check route handlers
type Handler struct{}

// New creates a new health Handler
func New() *Handler { return &Handler{} }

// Ping returns a simple health check response
// GET /api/ping
func (h *Handler) Ping(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"message": "pong",
	})
}

// Health returns detailed health check information
// GET /api/health
func (h *Handler) Health(c *fiber.Ctx) error {
	cfg := config.Get()

	// Check databases
	dbHealth := database.GetManager().HealthCheck()

	// Check Redis
	redisHealth := database.GetRedis().HealthCheck()

	// System info
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
	httpStatus := fiber.StatusOK
	if !allHealthy {
		status = "degraded"
		httpStatus = fiber.StatusServiceUnavailable
	}

	return c.Status(httpStatus).JSON(fiber.Map{
		"status":    status,
		"app_name":  cfg.App.Name,
		"env":       cfg.App.Env,
		"uptime":    time.Since(startTime).String(),
		"databases": dbHealth,
		"redis":     redisHealth,
		"memory": fiber.Map{
			"alloc_mb":       memStats.Alloc / 1024 / 1024,
			"sys_mb":         memStats.Sys / 1024 / 1024,
			"num_gc":         memStats.NumGC,
			"goroutines":     runtime.NumGoroutine(),
		},
		"timestamp": time.Now().UTC(),
	})
}
