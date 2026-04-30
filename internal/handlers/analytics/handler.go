package analytics

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains analytics route handlers
type Handler struct {
	svc services.AnalyticsService
}

// New creates a new analytics Handler
func New(svc services.AnalyticsService) *Handler {
	return &Handler{svc: svc}
}

// VisitorAnalytics returns the full analytics payload expected by the frontend.
// GET /api/dashboard/visitor-analytics?days=30
func (h *Handler) VisitorAnalytics(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)

	days := 30
	if d, err := fmt.Sscan(c.Query("days", "30"), &days); d == 0 || err != nil || days <= 0 || days > 365 {
		days = 30
	}

	data := h.svc.GetVisitorAnalytics(countryID, days)
	return utils.Success(c, "success", data)
}

// PruneAnalytics deletes old visitor data
// POST /api/dashboard/visitor-analytics/prune
func (h *Handler) PruneAnalytics(c *fiber.Ctx) error {
	type PruneRequest struct {
		Days int `json:"days"`
	}

	var req PruneRequest
	if err := c.BodyParser(&req); err != nil || req.Days == 0 {
		req.Days = 90
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	deleted := h.svc.PruneAnalytics(countryID, req.Days)

	return utils.Success(c, "تم حذف البيانات القديمة", services.PruneAnalyticsResponse{
		Deleted: deleted,
	})
}

// DashboardSummary returns the main dashboard data expected by the frontend.
// GET /api/dashboard
func (h *Handler) DashboardSummary(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)

	data := h.svc.GetDashboardSummary(countryID)
	return utils.Success(c, "success", data)
}

// ContentAnalytics returns content performance
// GET /api/dashboard/content-analytics
func (h *Handler) ContentAnalytics(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)

	data := h.svc.GetContentAnalytics(countryID)
	return utils.Success(c, "success", data)
}

// PerformanceSummary returns app performance metrics
// GET /api/dashboard/performance/summary
func (h *Handler) PerformanceSummary(c *fiber.Ctx) error {
	rdb := database.Redis()
	info, _ := rdb.GetInfo(c.Context())

	return utils.Success(c, "success", services.PerformanceSummaryResponse{
		RedisInfo: info,
		Timestamp: time.Now(),
	})
}

// PerformanceLive returns lightweight live metrics expected by the dashboard.
// GET /api/dashboard/performance/live
func (h *Handler) PerformanceLive(c *fiber.Ctx) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	total := int64(mem.Sys)
	used := int64(mem.Alloc)
	free := total - used
	if free < 0 {
		free = 0
	}

	usage := 0.0
	if total > 0 {
		usage = (float64(used) / float64(total)) * 100
	}

	return utils.Success(c, "success", fiber.Map{
		"cpu": fiber.Map{
			"usage": 0,
			"cores": runtime.NumCPU(),
			"load":  0,
		},
		"memory": fiber.Map{
			"total":            total,
			"free":             free,
			"used":             used,
			"usage_percentage": usage,
			"percentage":       usage,
		},
		"disk": fiber.Map{
			"total":            0,
			"free":             0,
			"used":             0,
			"usage_percentage": 0,
			"percentage":       0,
		},
		"timestamp": time.Now(),
	})
}

// PerformanceResponseTime measures a cheap internal Redis ping.
// GET /api/dashboard/performance/response-time
func (h *Handler) PerformanceResponseTime(c *fiber.Ctx) error {
	start := time.Now()
	_ = database.Redis().Default().Ping(c.Context()).Err()

	return utils.Success(c, "success", fiber.Map{
		"average_ms": time.Since(start).Milliseconds(),
	})
}

// PerformanceCache returns Redis cache hit ratio and size.
// GET /api/dashboard/performance/cache
func (h *Handler) PerformanceCache(c *fiber.Ctx) error {
	info, _ := database.Redis().GetInfo(c.Context())
	parsed := parseRedisInfo(info)

	hits := parseRedisInt(parsed["keyspace_hits"])
	misses := parseRedisInt(parsed["keyspace_misses"])
	total := hits + misses

	hitRatio := 0.0
	if total > 0 {
		hitRatio = (float64(hits) / float64(total)) * 100
	}

	cacheSize := parsed["used_memory_human"]
	if cacheSize == "" {
		cacheSize = "0 B"
	}

	return utils.Success(c, "success", fiber.Map{
		"hit_ratio":  hitRatio,
		"cache_size": cacheSize,
	})
}

// PerformanceRaw returns raw Redis and Go runtime metrics for debugging.
// GET /api/dashboard/performance/raw
func (h *Handler) PerformanceRaw(c *fiber.Ctx) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	info, _ := database.Redis().GetInfo(c.Context())

	return utils.Success(c, "success", fiber.Map{
		"redis_info": parseRedisInfo(info),
		"go": fiber.Map{
			"goroutines": runtime.NumGoroutine(),
			"alloc":      mem.Alloc,
			"sys":        mem.Sys,
			"num_gc":     mem.NumGC,
		},
		"timestamp": time.Now(),
	})
}

func parseRedisInfo(info string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		result[key] = value
	}
	return result
}

func parseRedisInt(value string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return n
}
