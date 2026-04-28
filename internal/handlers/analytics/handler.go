package analytics

import (
	"fmt"
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

	return utils.Success(c, "تم حذف البيانات القديمة", fiber.Map{
		"deleted": deleted,
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

	return utils.Success(c, "success", fiber.Map{
		"redis_info": info,
		"timestamp":  time.Now(),
	})
}

