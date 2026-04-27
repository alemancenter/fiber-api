package comments

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains comments and reactions route handlers
type Handler struct{}

// New creates a new comments Handler
func New() *Handler { return &Handler{} }

// List returns comments for a given country database
// GET /api/comments/:database
func (h *Handler) List(c *fiber.Ctx) error {
	dbCode := c.Params("database")
	db := database.GetManager().GetByCode(dbCode)
	pag := utils.GetPagination(c)

	var commentList []models.Comment
	var total int64

	query := db.Model(&models.Comment{}).Preload("User").
		Where("database = ?", dbCode)

	if commentableType := c.Query("type"); commentableType != "" {
		query = query.Where("commentable_type = ?", commentableType)
	}
	if commentableID := c.Query("id"); commentableID != "" {
		query = query.Where("commentable_id = ?", commentableID)
	}

	query.Count(&total)
	query.Order("created_at DESC").Limit(pag.PerPage).Offset(pag.Offset).Find(&commentList)

	return utils.Paginated(c, "success", commentList, pag.BuildMeta(total))
}

// CreateReaction creates an emoji reaction on a comment
// POST /api/reactions
func (h *Handler) CreateReaction(c *fiber.Ctx) error {
	type ReactionRequest struct {
		CommentID uint   `json:"comment_id" validate:"required"`
		Emoji     string `json:"emoji" validate:"required,max=20"`
	}

	var req ReactionRequest
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

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	// Upsert reaction
	reaction := models.Reaction{
		CommentID: req.CommentID,
		UserID:    user.ID,
		Emoji:     req.Emoji,
	}
	db.Where(models.Reaction{CommentID: req.CommentID, UserID: user.ID}).
		Assign(reaction).
		FirstOrCreate(&reaction)

	return utils.Created(c, "تم إضافة التفاعل بنجاح", reaction)
}

// DeleteReaction removes a reaction from a comment
// DELETE /api/reactions/:comment_id
func (h *Handler) DeleteReaction(c *fiber.Ctx) error {
	commentID, err := strconv.ParseUint(c.Params("comment_id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	db.Where("comment_id = ? AND user_id = ?", commentID, user.ID).Delete(&models.Reaction{})

	return utils.Success(c, "تم حذف التفاعل بنجاح", nil)
}

// GetReactions returns reactions for a comment
// GET /api/reactions/:comment_id
func (h *Handler) GetReactions(c *fiber.Ctx) error {
	commentID, err := strconv.ParseUint(c.Params("comment_id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var reactions []models.Reaction
	db.Preload("User").Where("comment_id = ?", commentID).Find(&reactions)

	return utils.Success(c, "success", reactions)
}

// DashboardList returns comments for dashboard management
// GET /api/dashboard/comments/:database
func (h *Handler) DashboardList(c *fiber.Ctx) error {
	return h.List(c)
}

// DashboardCreate creates a comment (dashboard)
// POST /api/dashboard/comments/:database
func (h *Handler) DashboardCreate(c *fiber.Ctx) error {
	dbCode := c.Params("database")
	db := database.GetManager().GetByCode(dbCode)

	type CreateRequest struct {
		Body            string `json:"body" validate:"required,min=1"`
		CommentableID   uint   `json:"commentable_id" validate:"required"`
		CommentableType string `json:"commentable_type" validate:"required"`
	}

	var req CreateRequest
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

	comment := models.Comment{
		Body:            utils.SanitizeInput(req.Body),
		UserID:          user.ID,
		CommentableID:   req.CommentableID,
		CommentableType: req.CommentableType,
		Database:        dbCode,
	}

	if err := db.Create(&comment).Error; err != nil {
		return utils.InternalError(c, "فشل إنشاء التعليق")
	}

	return utils.Created(c, "تم إنشاء التعليق بنجاح", comment)
}

// DashboardDelete deletes a comment (dashboard)
// DELETE /api/dashboard/comments/:database/:id
func (h *Handler) DashboardDelete(c *fiber.Ctx) error {
	dbCode := c.Params("database")
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	db := database.GetManager().GetByCode(dbCode)
	db.Delete(&models.Comment{}, id)

	return utils.Success(c, "تم حذف التعليق بنجاح", nil)
}
