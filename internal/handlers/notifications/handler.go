package notifications

import (
	"strconv"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Handler contains notifications route handlers
type Handler struct{}

// New creates a new notifications Handler
func New() *Handler { return &Handler{} }

// List returns paginated notifications for the current user
// GET /api/dashboard/notifications
func (h *Handler) List(c *fiber.Ctx) error {
	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	db := database.DB()
	pag := utils.GetPagination(c)

	var notifications []models.Notification
	var total int64

	query := db.Model(&models.Notification{}).
		Where("notifiable_type = ? AND notifiable_id = ?", "App\\Models\\User", user.ID)

	if unread := c.Query("unread"); unread == "1" {
		query = query.Where("read_at IS NULL")
	}

	query.Count(&total)
	query.Order("created_at DESC").Limit(pag.PerPage).Offset(pag.Offset).Find(&notifications)

	return utils.Paginated(c, "success", notifications, pag.BuildMeta(total))
}

// Latest returns the latest 10 notifications
// GET /api/dashboard/notifications/latest
func (h *Handler) Latest(c *fiber.Ctx) error {
	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	db := database.DB()
	var notifications []models.Notification
	db.Where("notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL",
		"App\\Models\\User", user.ID).
		Order("created_at DESC").
		Limit(10).
		Find(&notifications)

	var unreadCount int64
	db.Model(&models.Notification{}).
		Where("notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL",
			"App\\Models\\User", user.ID).
		Count(&unreadCount)

	return utils.Success(c, "success", fiber.Map{
		"notifications": notifications,
		"unread_count":  unreadCount,
	})
}

// MarkAsRead marks a notification as read
// POST /api/dashboard/notifications/:id/read
func (h *Handler) MarkAsRead(c *fiber.Ctx) error {
	id := c.Params("id")
	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	db := database.DB()
	now := time.Now()
	db.Model(&models.Notification{}).
		Where("id = ? AND notifiable_id = ?", id, user.ID).
		Update("read_at", now)

	return utils.Success(c, "تم قراءة الإشعار", nil)
}

// MarkAllRead marks all notifications as read
// POST /api/dashboard/notifications/read-all
func (h *Handler) MarkAllRead(c *fiber.Ctx) error {
	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	db := database.DB()
	now := time.Now()
	db.Model(&models.Notification{}).
		Where("notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL",
			"App\\Models\\User", user.ID).
		Update("read_at", now)

	return utils.Success(c, "تم تعليم جميع الإشعارات كمقروءة", nil)
}

// Create creates a new notification
// POST /api/dashboard/notifications
func (h *Handler) Create(c *fiber.Ctx) error {
	type CreateRequest struct {
		Type          string `json:"type" validate:"required"`
		NotifiableID  uint   `json:"notifiable_id" validate:"required"`
		Data          string `json:"data" validate:"required"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	db := database.DB()
	notification := models.Notification{
		ID:             uuid.New().String(),
		Type:           req.Type,
		NotifiableType: "App\\Models\\User",
		NotifiableID:   req.NotifiableID,
		Data:           req.Data,
	}

	if err := db.Create(&notification).Error; err != nil {
		return utils.InternalError(c, "فشل إنشاء الإشعار")
	}

	return utils.Created(c, "تم إنشاء الإشعار بنجاح", notification)
}

// Delete deletes a notification
// DELETE /api/dashboard/notifications/:id
func (h *Handler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	db := database.DB()
	db.Where("id = ? AND notifiable_id = ?", id, user.ID).Delete(&models.Notification{})

	return utils.Success(c, "تم حذف الإشعار", nil)
}

// Prune deletes old read notifications
// POST /api/dashboard/notifications/prune
func (h *Handler) Prune(c *fiber.Ctx) error {
	cutoff := time.Now().AddDate(0, 0, -30)
	db := database.DB()
	result := db.Where("read_at IS NOT NULL AND read_at < ?", cutoff).Delete(&models.Notification{})

	return utils.Success(c, "تم حذف الإشعارات القديمة", fiber.Map{
		"deleted": result.RowsAffected,
	})
}

// BulkAction performs a bulk action on notifications
// POST /api/dashboard/notifications/bulk
func (h *Handler) BulkAction(c *fiber.Ctx) error {
	type BulkRequest struct {
		Action string   `json:"action" validate:"required,oneof=read delete"`
		IDs    []string `json:"ids" validate:"required"`
	}

	var req BulkRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	db := database.DB()
	query := db.Where("id IN ? AND notifiable_id = ?", req.IDs, user.ID)

	switch req.Action {
	case "read":
		now := time.Now()
		query.Model(&models.Notification{}).Update("read_at", now)
	case "delete":
		query.Delete(&models.Notification{})
	default:
		return utils.BadRequest(c, "إجراء غير صحيح")
	}

	// Required import but not used directly
	_ = strconv.Itoa(0)

	return utils.Success(c, "تم تنفيذ الإجراء بنجاح", nil)
}
