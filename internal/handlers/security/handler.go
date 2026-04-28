package security

import (
	"strconv"
	"time"

	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains security route handlers
type Handler struct {
	svc services.SecurityService
}

// New creates a new security Handler
func New(svc services.SecurityService) *Handler {
	return &Handler{svc: svc}
}

// Stats returns security statistics
// GET /api/dashboard/security/stats
func (h *Handler) Stats(c *fiber.Ctx) error {
	totalLogs, criticalLogs, resolvedLogs, blockedIPs, trustedIPs, err := h.svc.GetStats()
	if err != nil {
		return utils.InternalError(c)
	}

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
	pag := utils.GetPagination(c)

	severity := c.Query("severity")
	eventType := c.Query("event_type")
	ip := c.Query("ip")
	resolved := c.Query("resolved")

	logs, total, err := h.svc.GetLogs(severity, eventType, ip, resolved, pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", logs, pag.BuildMeta(total))
}

// ResolveLog marks a security log as resolved
// POST /api/dashboard/security/logs/:id/resolve
func (h *Handler) ResolveLog(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	if err := h.svc.ResolveLog(id); err != nil {
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

	if err := h.svc.DeleteLog(id); err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "تم حذف السجل بنجاح", nil)
}

// DeleteAllLogs deletes all security logs
// DELETE /api/dashboard/security/logs
func (h *Handler) DeleteAllLogs(c *fiber.Ctx) error {
	if err := h.svc.DeleteAllLogs(); err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "تم حذف جميع السجلات بنجاح", nil)
}

// Overview returns a security overview
// GET /api/dashboard/security/overview
func (h *Handler) Overview(c *fiber.Ctx) error {
	last24h := time.Now().Add(-24 * time.Hour)
	last7d := time.Now().Add(-7 * 24 * time.Hour)

	last24hCount, last7dCount, totalAttacks, err := h.svc.GetOverviewStats(last24h, last7d)
	if err != nil {
		return utils.InternalError(c)
	}

	topIPs, err := h.svc.GetTopAttackingIPs(10)
	if err != nil {
		return utils.InternalError(c)
	}

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

	logs, count, err := h.svc.GetIPLogs(ip, 20)
	if err != nil {
		return utils.InternalError(c)
	}

	isBlocked := h.svc.IsBlocked(ip)
	isTrusted := h.svc.IsTrusted(ip)

	return utils.Success(c, "success", fiber.Map{
		"ip":           ip,
		"is_blocked":   isBlocked,
		"is_trusted":   isTrusted,
		"total_events": count,
		"recent_logs":  logs,
	})
}

// BlockIP blocks an IP address
// POST /api/dashboard/security/ip/:ip/block
func (h *Handler) BlockIP(c *fiber.Ctx) error {
	ip := c.Params("ip")
	var req struct {
		Reason string `json:"reason" validate:"required"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صالحة")
	}

	userID := c.Locals("user_id").(uint)

	if err := h.svc.BlockIP(ip, req.Reason, &userID); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم حظر عنوان IP بنجاح", nil)
}

// UnblockIP unblocks an IP address
// POST /api/dashboard/security/ip/:ip/unblock
func (h *Handler) UnblockIP(c *fiber.Ctx) error {
	ip := c.Params("ip")

	if err := h.svc.UnblockIP(ip); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم إلغاء حظر عنوان IP بنجاح", nil)
}

// TrustIP marks an IP as trusted
// POST /api/dashboard/security/ip/:ip/trust
func (h *Handler) TrustIP(c *fiber.Ctx) error {
	ip := c.Params("ip")
	var req struct {
		Note string `json:"note"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صالحة")
	}

	userID := c.Locals("user_id").(uint)

	if err := h.svc.TrustIP(ip, req.Note, &userID); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم إضافة عنوان IP للموثوقين بنجاح", nil)
}

// UntrustIP removes an IP from trusted list
// POST /api/dashboard/security/ip/:ip/untrust
func (h *Handler) UntrustIP(c *fiber.Ctx) error {
	ip := c.Params("ip")

	if err := h.svc.UntrustIP(ip); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم إزالة عنوان IP من الموثوقين بنجاح", nil)
}

// BlockedIPs lists all blocked IPs
// GET /api/dashboard/security/blocked-ips
func (h *Handler) BlockedIPs(c *fiber.Ctx) error {
	pag := utils.GetPagination(c)

	blocked, total, err := h.svc.GetBlockedIPs(pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", blocked, pag.BuildMeta(total))
}

// TrustedIPs lists all trusted IPs
// GET /api/dashboard/security/trusted-ips
func (h *Handler) TrustedIPs(c *fiber.Ctx) error {
	pag := utils.GetPagination(c)

	trusted, total, err := h.svc.GetTrustedIPs(pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", trusted, pag.BuildMeta(total))
}

// Analytics returns security analytics
// GET /api/dashboard/security/analytics
func (h *Handler) Analytics(c *fiber.Ctx) error {
	bySeverity, err := h.svc.GetAnalyticsBySeverity()
	if err != nil {
		return utils.InternalError(c)
	}

	byEventType, err := h.svc.GetAnalyticsByEventType(10)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", fiber.Map{
		"by_severity":   bySeverity,
		"by_event_type": byEventType,
	})
}

// TopRoutes returns the most targeted routes
// GET /api/dashboard/security/analytics/routes
func (h *Handler) TopRoutes(c *fiber.Ctx) error {
	routes, err := h.svc.GetTopRoutes(20)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", routes)
}

// GeoDistribution returns geographic distribution of events
// GET /api/dashboard/security/analytics/geo
func (h *Handler) GeoDistribution(c *fiber.Ctx) error {
	geo, err := h.svc.GetGeoDistribution(20)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", geo)
}
