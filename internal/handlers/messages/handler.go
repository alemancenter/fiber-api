package messages

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains messages route handlers
type Handler struct {
	svc services.MessageService
}

// New creates a new messages Handler
func New(svc services.MessageService) *Handler {
	return &Handler{
		svc: svc,
	}
}

func getUser(c *fiber.Ctx) *models.User {
	user, _ := c.Locals("user").(*models.User)
	return user
}

// Inbox returns received messages for the current user.
// GET /api/dashboard/messages/inbox
func (h *Handler) Inbox(c *fiber.Ctx) error {
	user := getUser(c)
	if user == nil {
		return utils.Unauthorized(c)
	}

	pag := utils.GetPagination(c)
	msgs, total, err := h.svc.ListInbox(user.ID, pag.Offset, pag.PerPage)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", msgs, pag.BuildMeta(total))
}

// Sent returns messages sent by the current user.
// GET /api/dashboard/messages/sent
func (h *Handler) Sent(c *fiber.Ctx) error {
	user := getUser(c)
	if user == nil {
		return utils.Unauthorized(c)
	}

	pag := utils.GetPagination(c)
	msgs, total, err := h.svc.ListSent(user.ID, pag.Offset, pag.PerPage)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", msgs, pag.BuildMeta(total))
}

// Drafts returns draft messages for the current user.
// GET /api/dashboard/messages/drafts
func (h *Handler) Drafts(c *fiber.Ctx) error {
	user := getUser(c)
	if user == nil {
		return utils.Unauthorized(c)
	}

	pag := utils.GetPagination(c)
	msgs, total, err := h.svc.ListDrafts(user.ID, pag.Offset, pag.PerPage)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", msgs, pag.BuildMeta(total))
}

// Send sends a new message.
// POST /api/dashboard/messages/send
func (h *Handler) Send(c *fiber.Ctx) error {
	user := getUser(c)
	if user == nil {
		return utils.Unauthorized(c)
	}

	type SendRequest struct {
		RecipientID uint   `json:"recipient_id" validate:"required"`
		Subject     string `json:"subject"`
		Body        string `json:"body" validate:"required,min=1"`
	}

	var req SendRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}
	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	msg, err := h.svc.SendMessage(user.ID, req.RecipientID, req.Subject, req.Body)
	if err != nil {
		return utils.InternalError(c, "فشل إرسال الرسالة")
	}

	return utils.Created(c, "تم إرسال الرسالة بنجاح", msg)
}

// Draft saves a draft message.
// POST /api/dashboard/messages/draft
func (h *Handler) Draft(c *fiber.Ctx) error {
	user := getUser(c)
	if user == nil {
		return utils.Unauthorized(c)
	}

	type DraftRequest struct {
		RecipientID uint   `json:"recipient_id"`
		Subject     string `json:"subject"`
		Body        string `json:"body" validate:"required"`
	}

	var req DraftRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	msg, err := h.svc.SaveDraft(user.ID, req.RecipientID, req.Subject, req.Body)
	if err != nil {
		return utils.InternalError(c, "فشل حفظ المسودة")
	}

	return utils.Created(c, "تم حفظ المسودة بنجاح", msg)
}

// Get returns a single message.
// GET /api/dashboard/messages/:id
func (h *Handler) Get(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	user := getUser(c)
	if user == nil {
		return utils.Unauthorized(c)
	}

	msg, err := h.svc.GetMessage(id, user.ID)
	if err != nil {
		return utils.NotFound(c)
	}

	return utils.Success(c, "success", msg)
}

// MarkAsRead marks a message as read.
// POST /api/dashboard/messages/:id/read
func (h *Handler) MarkAsRead(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	user := getUser(c)
	if user == nil {
		return utils.Unauthorized(c)
	}

	if err := h.svc.MarkAsRead(id, user.ID); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم تعليم الرسالة كمقروءة", nil)
}

// ToggleImportant toggles the important flag on a message.
// POST /api/dashboard/messages/:id/important
func (h *Handler) ToggleImportant(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	user := getUser(c)
	if user == nil {
		return utils.Unauthorized(c)
	}

	if err := h.svc.ToggleImportant(id, user.ID); err != nil {
		return utils.NotFound(c)
	}

	return utils.Success(c, "تم تحديث حالة الرسالة", nil)
}

// Delete soft-deletes a message.
// DELETE /api/dashboard/messages/:id
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	user := getUser(c)
	if user == nil {
		return utils.Unauthorized(c)
	}

	if err := h.svc.DeleteMessage(id, user.ID); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم حذف الرسالة", nil)
}
