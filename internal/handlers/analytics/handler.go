package analytics

import (
	"fmt"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains analytics route handlers
type Handler struct{}

// New creates a new analytics Handler
func New() *Handler { return &Handler{} }

// VisitorAnalytics returns visitor analytics data
// GET /api/dashboard/visitor-analytics
func (h *Handler) VisitorAnalytics(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	countryCode, _ := c.Locals("country_code").(string)

	period := c.Query("period", "30")
	days := 30
	if p, err := fmt.Sscan(period, &days); p > 0 && err == nil && days > 0 && days <= 365 {
	} else {
		days = 30
	}

	since := time.Now().AddDate(0, 0, -days)

	var totalVisits int64
	db.Model(&models.VisitorTracking{}).
		Where("database = ? AND created_at >= ?", countryCode, since).
		Count(&totalVisits)

	type UniqueCount struct{ Count int64 }
	var uniqueResult UniqueCount
	db.Model(&models.VisitorTracking{}).
		Select("COUNT(DISTINCT ip_address) as count").
		Where("database = ? AND created_at >= ?", countryCode, since).
		Scan(&uniqueResult)

	type DailyCount struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	var dailyCounts []DailyCount
	db.Model(&models.VisitorTracking{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("database = ? AND created_at >= ?", countryCode, since).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&dailyCounts)

	type PageCount struct {
		Page  string `json:"page"`
		Count int64  `json:"count"`
	}
	var topPages []PageCount
	db.Model(&models.VisitorTracking{}).
		Select("page, COUNT(*) as count").
		Where("database = ? AND created_at >= ?", countryCode, since).
		Group("page").
		Order("count DESC").
		Limit(10).
		Scan(&topPages)

	return utils.Success(c, "success", fiber.Map{
		"total_visits":    totalVisits,
		"unique_visitors": uniqueResult.Count,
		"daily_counts":    dailyCounts,
		"top_pages":       topPages,
		"period_days":     days,
	})
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

	cutoff := time.Now().AddDate(0, 0, -req.Days)
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	countryCode, _ := c.Locals("country_code").(string)

	result := db.Where("database = ? AND created_at < ?", countryCode, cutoff).
		Delete(&models.VisitorTracking{})

	return utils.Success(c, "تم حذف البيانات القديمة", fiber.Map{
		"deleted": result.RowsAffected,
	})
}

// ContentAnalytics returns content performance
// GET /api/dashboard/content-analytics
func (h *Handler) ContentAnalytics(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	type ArticleView struct {
		ID         uint   `json:"id"`
		Title      string `json:"title"`
		VisitCount int    `json:"visit_count"`
	}
	var topArticles []ArticleView
	db.Model(&models.Article{}).
		Select("id, title, visit_count").
		Where("status = ?", 1).
		Order("visit_count DESC").
		Limit(10).
		Scan(&topArticles)

	type PostView struct {
		ID    uint   `json:"id"`
		Title string `json:"title"`
		Views int    `json:"views"`
	}
	var topPosts []PostView
	db.Model(&models.Post{}).
		Select("id, title, views").
		Where("is_active = ?", true).
		Order("views DESC").
		Limit(10).
		Scan(&topPosts)

	var publishedArticles, draftArticles int64
	db.Model(&models.Article{}).Where("status = ?", 1).Count(&publishedArticles)
	db.Model(&models.Article{}).Where("status = ?", 0).Count(&draftArticles)

	return utils.Success(c, "success", fiber.Map{
		"top_articles":       topArticles,
		"top_posts":          topPosts,
		"published_articles": publishedArticles,
		"draft_articles":     draftArticles,
	})
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
