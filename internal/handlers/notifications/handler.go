package notifications

import (
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains notifications route handlers
type Handler struct {
	svc services.NotificationService
}

// New creates a new notifications Handler
func New(svc services.NotificationService) *Handler {
	return &Handler{
		svc: svc,
	}
}

// List returns paginated notifications for the current user
// GET /api/dashboard/notifications
func (h *Handler) List(c *fiber.Ctx) error {
	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	pag := utils.GetPagination(c)
	unreadOnly := c.Query("unread") == "1"

	notifications, total, err := h.svc.List(user.ID, unreadOnly, pag.Offset, pag.PerPage)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", notifications, pag.BuildMeta(total))
}

// Latest returns the latest 10 notifications
// GET /api/dashboard/notifications/latest
func (h *Handler) Latest(c *fiber.Ctx) error {
	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	notifications, unreadCount, err := h.svc.GetLatest(user.ID, 10)
	if err != nil {
		return utils.InternalError(c)
	}

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

	if err := h.svc.MarkAsRead(id, user.ID); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم قراءة الإشعار", nil)
}

// MarkAllRead marks all notifications as read
// POST /api/dashboard/notifications/read-all
func (h *Handler) MarkAllRead(c *fiber.Ctx) error {
	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	if err := h.svc.MarkAllRead(user.ID); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم تعليم جميع الإشعارات كمقروءة", nil)
}

// Create creates a new notification
// POST /api/dashboard/notifications
func (h *Handler) Create(c *fiber.Ctx) error {
	type CreateRequest struct {
		Type         string `json:"type" validate:"required"`
		NotifiableID uint   `json:"notifiable_id" validate:"required"`
		Data         string `json:"data" validate:"required"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	notification, err := h.svc.Create(req.Type, req.NotifiableID, req.Data)
	if err != nil {
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

	if err := h.svc.Delete(id, user.ID); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم حذف الإشعار", nil)
}

// Prune deletes old read notifications
// POST /api/dashboard/notifications/prune
func (h *Handler) Prune(c *fiber.Ctx) error {
	deletedCount, err := h.svc.Prune(30)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم حذف الإشعارات القديمة", fiber.Map{
		"deleted": deletedCount,
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

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	if err := h.svc.BulkAction(req.Action, req.IDs, user.ID); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم تنفيذ الإجراء بنجاح", nil)
}