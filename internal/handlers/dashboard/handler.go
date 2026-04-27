package dashboard

import (
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains dashboard overview route handlers
type Handler struct{}

// New creates a new dashboard Handler
func New() *Handler { return &Handler{} }

// Home returns dashboard overview statistics
// GET /api/dashboard
func (h *Handler) Home(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	mainDB := database.DB()

	// Stats counts
	var articles, posts, users, comments, files int64
	var blockedIPs, securityLogs int64

	db.Model(&models.Article{}).Count(&articles)
	db.Model(&models.Post{}).Count(&posts)
	mainDB.Model(&models.User{}).Count(&users)
	db.Model(&models.Comment{}).Count(&comments)
	db.Model(&models.File{}).Count(&files)
	mainDB.Model(&models.BlockedIP{}).Count(&blockedIPs)
	mainDB.Model(&models.SecurityLog{}).Where("severity IN ?", []string{"danger", "critical"}).Count(&securityLogs)

	// Recent activity
	var recentActivity []models.ActivityLog
	mainDB.Order("created_at DESC").Limit(10).Find(&recentActivity)

	// Recent users
	var recentUsers []models.User
	mainDB.Select("id, name, email, created_at, status").
		Order("created_at DESC").Limit(5).Find(&recentUsers)

	// Online users (active in last 5 minutes)
	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	var onlineCount int64
	mainDB.Model(&models.User{}).Where("last_activity >= ?", fiveMinAgo).Count(&onlineCount)

	return utils.Success(c, "success", fiber.Map{
		"stats": fiber.Map{
			"articles":       articles,
			"posts":          posts,
			"users":          users,
			"comments":       comments,
			"files":          files,
			"blocked_ips":    blockedIPs,
			"security_logs":  securityLogs,
			"online_users":   onlineCount,
		},
		"recent_activity": recentActivity,
		"recent_users":    recentUsers,
	})
}

// Activities returns the activity log
// GET /api/dashboard/activities
func (h *Handler) Activities(c *fiber.Ctx) error {
	db := database.DB()
	pag := utils.GetPagination(c)

	var activities []models.ActivityLog
	var total int64

	query := db.Model(&models.ActivityLog{})

	if logName := c.Query("log_name"); logName != "" {
		query = query.Where("log_name = ?", logName)
	}
	if causerID := c.Query("causer_id"); causerID != "" {
		query = query.Where("causer_id = ?", causerID)
	}

	query.Count(&total)
	query.Order("created_at DESC").Limit(pag.PerPage).Offset(pag.Offset).Find(&activities)

	return utils.Paginated(c, "success", activities, pag.BuildMeta(total))
}

// CleanActivities removes old activity logs
// DELETE /api/dashboard/activities/clean
func (h *Handler) CleanActivities(c *fiber.Ctx) error {
	cutoff := time.Now().AddDate(0, -3, 0) // 3 months ago
	db := database.DB()
	result := db.Where("created_at < ?", cutoff).Delete(&models.ActivityLog{})

	return utils.Success(c, "تم تنظيف السجلات القديمة", fiber.Map{
		"deleted": result.RowsAffected,
	})
}
