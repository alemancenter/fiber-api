package security

import (
	"strconv"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains security route handlers
type Handler struct {
	svc *services.SecurityService
}

// New creates a new security Handler
func New() *Handler {
	return &Handler{svc: services.NewSecurityService()}
}

// Stats returns security statistics
// GET /api/dashboard/security/stats
func (h *Handler) Stats(c *fiber.Ctx) error {
	db := database.DB()

	var totalLogs, criticalLogs, resolvedLogs int64
	var blockedIPs, trustedIPs int64

	db.Model(&models.SecurityLog{}).Count(&totalLogs)
	db.Model(&models.SecurityLog{}).Where("severity = ?", models.SeverityCritical).Count(&criticalLogs)
	db.Model(&models.SecurityLog{}).Where("is_resolved = ?", true).Count(&resolvedLogs)
	db.Model(&models.BlockedIP{}).Count(&blockedIPs)
	db.Model(&models.TrustedIP{}).Count(&trustedIPs)

	return utils.Success(c, "success", fiber.Map{
		"total_logs":    totalLogs,
		"critical_logs": criticalLogs,
		"resolved_logs": resolvedLogs,
		"blocked_ips":   blockedIPs,
		"trusted_ips":   trustedIPs,
	})
}

// Logs returns paginated security logs
// GET /api/dashboard/security/logs
func (h *Handler) Logs(c *fiber.Ctx) error {
	db := database.DB()
	pag := utils.GetPagination(c)

	var logs []models.SecurityLog
	var total int64

	query := db.Model(&models.SecurityLog{})

	if severity := c.Query("severity"); severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if eventType := c.Query("event_type"); eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if ip := c.Query("ip"); ip != "" {
		query = query.Where("ip_address = ?", ip)
	}
	if resolved := c.Query("resolved"); resolved == "1" {
		query = query.Where("is_resolved = ?", true)
	} else if resolved == "0" {
		query = query.Where("is_resolved = ?", false)
	}

	query.Count(&total)
	query.Order("created_at DESC").Limit(pag.PerPage).Offset(pag.Offset).Find(&logs)

	return utils.Paginated(c, "success", logs, pag.BuildMeta(total))
}

// ResolveLog marks a security log as resolved
// POST /api/dashboard/security/logs/:id/resolve
func (h *Handler) ResolveLog(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	db := database.DB()
	now := time.Now()
	result := db.Model(&models.SecurityLog{}).Where("id = ?", id).Updates(map[string]interface{}{
		"is_resolved": true,
		"resolved_at": now,
	})

	if result.RowsAffected == 0 {
		return utils.NotFound(c)
	}

	return utils.Success(c, "تم حل السجل بنجاح", nil)
}

// DeleteLog deletes a security log
// DELETE /api/dashboard/security/logs/:id
func (h *Handler) DeleteLog(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	db := database.DB()
	db.Delete(&models.SecurityLog{}, id)
	return utils.Success(c, "تم حذف السجل بنجاح", nil)
}

// DeleteAllLogs deletes all security logs
// DELETE /api/dashboard/security/logs
func (h *Handler) DeleteAllLogs(c *fiber.Ctx) error {
	db := database.DB()
	db.Where("1 = 1").Delete(&models.SecurityLog{})
	return utils.Success(c, "تم حذف جميع السجلات بنجاح", nil)
}

// Overview returns a security overview
// GET /api/dashboard/security/overview
func (h *Handler) Overview(c *fiber.Ctx) error {
	db := database.DB()
	last24h := time.Now().Add(-24 * time.Hour)
	last7d := time.Now().Add(-7 * 24 * time.Hour)

	var last24hCount, last7dCount, totalAttacks int64
	db.Model(&models.SecurityLog{}).Where("created_at >= ?", last24h).Count(&last24hCount)
	db.Model(&models.SecurityLog{}).Where("created_at >= ?", last7d).Count(&last7dCount)
	db.Model(&models.SecurityLog{}).Where("attack_type IS NOT NULL").Count(&totalAttacks)

	// Top attacking IPs
	type IPCount struct {
		IPAddress string `json:"ip_address"`
		Count     int64  `json:"count"`
	}
	var topIPs []IPCount
	db.Model(&models.SecurityLog{}).
		Select("ip_address, COUNT(*) as count").
		Group("ip_address").
		Order("count DESC").
		Limit(10).
		Scan(&topIPs)

	return utils.Success(c, "success", fiber.Map{
		"last_24h_events": last24hCount,
		"last_7d_events":  last7dCount,
		"total_attacks":   totalAttacks,
		"top_ips":         topIPs,
	})
}

// IPDetails returns details about a specific IP
// GET /api/dashboard/security/ip/:ip
func (h *Handler) IPDetails(c *fiber.Ctx) error {
	ip := c.Params("ip")
	db := database.DB()

	var logs []models.SecurityLog
	var count int64
	db.Model(&models.SecurityLog{}).Where("ip_address = ?", ip).Count(&count)
	db.Where("ip_address = ?", ip).Order("created_at DESC").Limit(20).Find(&logs)

	isBlocked := h.svc.IsBlocked(ip)
	isTrusted := h.svc.IsTrusted(ip)

	return utils.Success(c, "success", fiber.Map{
		"ip":          ip,
		"is_blocked":  isBlocked,
		"is_trusted":  isTrusted,
		"total_events": count,
		"recent_logs": logs,
	})
}

// BlockIP blocks an IP address
// POST /api/dashboard/security/ip/block
func (h *Handler) BlockIP(c *fiber.Ctx) error {
	type BlockRequest struct {
		IP     string `json:"ip" validate:"required"`
		Reason string `json:"reason"`
	}

	var req BlockRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	user, _ := c.Locals("user").(*models.User)
	var userID *uint
	if user != nil {
		uid := user.ID
		userID = &uid
	}

	if err := h.svc.BlockIP(req.IP, req.Reason, userID); err != nil {
		return utils.InternalError(c, "فشل حجب عنوان IP")
	}

	return utils.Success(c, "تم حجب عنوان IP بنجاح", nil)
}

// UnblockIP removes an IP block
// POST /api/dashboard/security/ip/unblock
func (h *Handler) UnblockIP(c *fiber.Ctx) error {
	type UnblockRequest struct {
		IP string `json:"ip" validate:"required"`
	}

	var req UnblockRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	h.svc.UnblockIP(req.IP)
	return utils.Success(c, "تم رفع الحجب عن عنوان IP", nil)
}

// TrustIP marks an IP as trusted
// POST /api/dashboard/security/ip/trust
func (h *Handler) TrustIP(c *fiber.Ctx) error {
	type TrustRequest struct {
		IP   string `json:"ip" validate:"required"`
		Note string `json:"note"`
	}

	var req TrustRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	user, _ := c.Locals("user").(*models.User)
	var userID *uint
	if user != nil {
		uid := user.ID
		userID = &uid
	}

	if err := h.svc.TrustIP(req.IP, req.Note, userID); err != nil {
		return utils.InternalError(c, "فشل إضافة عنوان IP للقائمة الموثوقة")
	}

	return utils.Success(c, "تم إضافة عنوان IP للقائمة الموثوقة", nil)
}

// UntrustIP removes IP from trusted list
// POST /api/dashboard/security/ip/untrust
func (h *Handler) UntrustIP(c *fiber.Ctx) error {
	type UntrustRequest struct {
		IP string `json:"ip" validate:"required"`
	}

	var req UntrustRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	h.svc.UntrustIP(req.IP)
	return utils.Success(c, "تم إزالة عنوان IP من القائمة الموثوقة", nil)
}

// BlockedIPs lists all blocked IPs
// GET /api/dashboard/security/blocked-ips
func (h *Handler) BlockedIPs(c *fiber.Ctx) error {
	db := database.DB()
	pag := utils.GetPagination(c)

	var blocked []models.BlockedIP
	var total int64

	db.Model(&models.BlockedIP{}).Count(&total)
	db.Order("created_at DESC").Limit(pag.PerPage).Offset(pag.Offset).Find(&blocked)

	return utils.Paginated(c, "success", blocked, pag.BuildMeta(total))
}

// TrustedIPs lists all trusted IPs
// GET /api/dashboard/security/trusted-ips
func (h *Handler) TrustedIPs(c *fiber.Ctx) error {
	db := database.DB()
	pag := utils.GetPagination(c)

	var trusted []models.TrustedIP
	var total int64

	db.Model(&models.TrustedIP{}).Count(&total)
	db.Order("created_at DESC").Limit(pag.PerPage).Offset(pag.Offset).Find(&trusted)

	return utils.Paginated(c, "success", trusted, pag.BuildMeta(total))
}

// Analytics returns security analytics
// GET /api/dashboard/security/analytics
func (h *Handler) Analytics(c *fiber.Ctx) error {
	db := database.DB()

	type SeverityCount struct {
		Severity string `json:"severity"`
		Count    int64  `json:"count"`
	}
	var bySeverity []SeverityCount
	db.Model(&models.SecurityLog{}).
		Select("severity, COUNT(*) as count").
		Group("severity").
		Scan(&bySeverity)

	type EventTypeCount struct {
		EventType string `json:"event_type"`
		Count     int64  `json:"count"`
	}
	var byEventType []EventTypeCount
	db.Model(&models.SecurityLog{}).
		Select("event_type, COUNT(*) as count").
		Group("event_type").
		Order("count DESC").
		Limit(10).
		Scan(&byEventType)

	return utils.Success(c, "success", fiber.Map{
		"by_severity":   bySeverity,
		"by_event_type": byEventType,
	})
}

// TopRoutes returns the most targeted routes
// GET /api/dashboard/security/analytics/routes
func (h *Handler) TopRoutes(c *fiber.Ctx) error {
	db := database.DB()

	type RouteCount struct {
		Route string `json:"route"`
		Count int64  `json:"count"`
	}
	var routes []RouteCount
	db.Model(&models.SecurityLog{}).
		Select("route, COUNT(*) as count").
		Where("route IS NOT NULL").
		Group("route").
		Order("count DESC").
		Limit(20).
		Scan(&routes)

	return utils.Success(c, "success", routes)
}

// GeoDistribution returns geographic distribution of events
// GET /api/dashboard/security/analytics/geo
func (h *Handler) GeoDistribution(c *fiber.Ctx) error {
	db := database.DB()

	type GeoCount struct {
		CountryCode string `json:"country_code"`
		Count       int64  `json:"count"`
	}
	var geo []GeoCount
	db.Model(&models.SecurityLog{}).
		Select("country_code, COUNT(*) as count").
		Where("country_code IS NOT NULL").
		Group("country_code").
		Order("count DESC").
		Limit(20).
		Scan(&geo)

	return utils.Success(c, "success", geo)
}
