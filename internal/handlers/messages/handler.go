package messages

import (
	"strconv"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains messages route handlers
type Handler struct{}

// New creates a new messages Handler
func New() *Handler { return &Handler{} }

// Inbox returns received messages
// GET /api/dashboard/messages/inbox
func (h *Handler) Inbox(c *fiber.Ctx) error {
	user := getUser(c)
	if user == nil {
		return utils.Unauthorized(c)
	}

	db := database.DB()
	pag := utils.GetPagination(c)
	var messages []models.Message
	var total int64

	query := db.Model(&models.Message{}).
		Preload("Sender").
		Where("recipient_id = ? AND is_draft = ?", user.ID, false)

	query.Count(&total)
	query.Order("created_at DESC").Limit(pag.PerPage).Offset(pag.Offset).Find(&messages)

	return utils.Paginated(c, "success", messages, pag.BuildMeta(total))
}

// Sent returns sent messages
// GET /api/dashboard/messages/sent
func (h *Handler) Sent(c *fiber.Ctx) error {
	user := getUser(c)
	if user == nil {
		return utils.Unauthorized(c)
	}

	db := database.DB()
	pag := utils.GetPagination(c)
	var messages []models.Message
	var total int64

	query := db.Model(&models.Message{}).
		Preload("Recipient").
		Where("sender_id = ? AND is_draft = ?", user.ID, false)

	query.Count(&total)
	query.Order("created_at DESC").Limit(pag.PerPage).Offset(pag.Offset).Find(&messages)

	return utils.Paginated(c, "success", messages, pag.BuildMeta(total))
}

// Drafts returns draft messages
// GET /api/dashboard/messages/drafts
func (h *Handler) Drafts(c *fiber.Ctx) error {
	user := getUser(c)
	if user == nil {
		return utils.Unauthorized(c)
	}

	db := database.DB()
	pag := utils.GetPagination(c)
	var messages []models.Message
	var total int64

	db.Model(&models.Message{}).Where("sender_id = ? AND is_draft = ?", user.ID, true).Count(&total)
	db.Preload("Recipient").
		Where("sender_id = ? AND is_draft = ?", user.ID, true).
		Order("created_at DESC").
		Limit(pag.PerPage).Offset(pag.Offset).
		Find(&messages)

	return utils.Paginated(c, "success", messages, pag.BuildMeta(total))
}

// Send sends a message
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

	db := database.DB()
	msg := models.Message{
		SenderID:    user.ID,
		RecipientID: req.RecipientID,
		Body:        req.Body,
	}
	if req.Subject != "" {
		msg.Subject = &req.Subject
	}

	if err := db.Create(&msg).Error; err != nil {
		return utils.InternalError(c, "فشل إرسال الرسالة")
	}

	return utils.Created(c, "تم إرسال الرسالة بنجاح", msg)
}

// Draft saves a draft message
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

	db := database.DB()
	msg := models.Message{
		SenderID: user.ID,
		Body:     req.Body,
		IsDraft:  true,
	}
	if req.RecipientID > 0 {
		msg.RecipientID = req.RecipientID
	}
	if req.Subject != "" {
		msg.Subject = &req.Subject
	}

	if err := db.Create(&msg).Error; err != nil {
		return utils.InternalError(c, "فشل حفظ المسودة")
	}

	return utils.Created(c, "تم حفظ المسودة بنجاح", msg)
}

// Get returns a single message
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

	db := database.DB()
	var msg models.Message
	if err := db.Preload("Sender").Preload("Recipient").
		Where("(sender_id = ? OR recipient_id = ?) AND id = ?", user.ID, user.ID, id).
		First(&msg).Error; err != nil {
		return utils.NotFound(c)
	}

	// Auto-mark as read if recipient
	if msg.RecipientID == user.ID && !msg.IsRead {
		now := time.Now()
		db.Model(&msg).Updates(map[string]interface{}{"is_read": true, "read_at": now})
	}

	return utils.Success(c, "success", msg)
}

// MarkAsRead marks a message as read
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

	db := database.DB()
	now := time.Now()
	db.Model(&models.Message{}).
		Where("id = ? AND recipient_id = ?", id, user.ID).
		Updates(map[string]interface{}{"is_read": true, "read_at": now})

	return utils.Success(c, "تم تعليم الرسالة كمقروءة", nil)
}

// ToggleImportant toggles the important flag
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

	db := database.DB()
	var msg models.Message
	if err := db.Where("(sender_id = ? OR recipient_id = ?) AND id = ?",
		user.ID, user.ID, id).First(&msg).Error; err != nil {
		return utils.NotFound(c)
	}

	db.Model(&msg).Update("is_important", !msg.IsImportant)
	return utils.Success(c, "تم تحديث حالة الرسالة", nil)
}

// Delete deletes a message
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

	db := database.DB()
	db.Where("(sender_id = ? OR recipient_id = ?) AND id = ?",
		user.ID, user.ID, id).Delete(&models.Message{})

	return utils.Success(c, "تم حذف الرسالة", nil)
}

func getUser(c *fiber.Ctx) *models.User {
	user, _ := c.Locals("user").(*models.User)
	return user
}
