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
// @Summary List Notifications
// @Description Returns a paginated list of notifications for the authenticated user
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Param unread query string false "Filter by unread status (1 for true)"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} utils.APIResponse{data=[]models.Notification}
// @Failure 401 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/notifications [get]
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
// @Summary Latest Notifications
// @Description Returns the 10 most recent notifications and the total unread count
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Success 200 {object} utils.APIResponse{data=services.LatestNotificationsResponse}
// @Failure 401 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/notifications/latest [get]
func (h *Handler) Latest(c *fiber.Ctx) error {
	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	notifications, unreadCount, err := h.svc.GetLatest(user.ID, 10)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", services.LatestNotificationsResponse{
		Notifications: notifications,
		UnreadCount:   unreadCount,
	})
}

// MarkAsRead marks a notification as read
// @Summary Mark Notification Read
// @Description Mark a specific notification as read
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} utils.APIResponse
// @Failure 401 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/notifications/{id}/read [post]
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
// @Summary Mark All Notifications Read
// @Description Mark all notifications as read for the authenticated user
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Success 200 {object} utils.APIResponse
// @Failure 401 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/notifications/read-all [post]
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

type CreateNotificationRequest struct {
	Type         string `json:"type" validate:"required"`
	NotifiableID uint   `json:"notifiable_id" validate:"required"`
	Data         string `json:"data" validate:"required"`
}

// Create creates a new notification
// @Summary Create Notification
// @Description Programmatically create a new notification for a user
// @Tags Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Param request body CreateNotificationRequest true "Notification payload"
// @Success 201 {object} utils.APIResponse{data=models.Notification}
// @Failure 400 {object} utils.APIResponse
// @Failure 422 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/notifications [post]
func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateNotificationRequest
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
// @Summary Delete Notification
// @Description Delete a specific notification by ID
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} utils.APIResponse
// @Failure 401 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/notifications/{id} [delete]
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
// @Summary Prune Notifications
// @Description Delete old, already-read notifications (e.g., older than 30 days)
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Success 200 {object} utils.APIResponse{data=services.PruneNotificationsResponse}
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/notifications/prune [post]
func (h *Handler) Prune(c *fiber.Ctx) error {
	deletedCount, err := h.svc.Prune(30)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم حذف الإشعارات القديمة", services.PruneNotificationsResponse{
		Deleted: deletedCount,
	})
}

type BulkActionRequest struct {
	Action string   `json:"action" validate:"required,oneof=read delete"`
	IDs    []string `json:"ids" validate:"required"`
}

// BulkAction performs a bulk action on notifications
// @Summary Bulk Action on Notifications
// @Description Perform an action (e.g., read, delete) on multiple notifications
// @Tags Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Param request body BulkActionRequest true "Bulk action payload"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Failure 401 {object} utils.APIResponse
// @Failure 422 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/notifications/bulk [post]
func (h *Handler) BulkAction(c *fiber.Ctx) error {
	var req BulkActionRequest
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
