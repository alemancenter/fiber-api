package posts

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Handler contains posts route handlers
type Handler struct{}

// New creates a new posts Handler
func New() *Handler { return &Handler{} }

// List returns a paginated list of published posts
// GET /api/posts
func (h *Handler) List(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	pag := utils.GetPagination(c)

	var postList []models.Post
	var total int64

	query := db.Model(&models.Post{}).Preload("Category").Preload("Author").
		Where("is_active = ?", true)

	if catID := c.Query("category_id"); catID != "" {
		query = query.Where("category_id = ?", catID)
	}
	if search := c.Query("search"); search != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if featured := c.Query("featured"); featured == "1" {
		query = query.Where("is_featured = ?", true)
	}

	query.Count(&total)
	query.Order("created_at DESC").Limit(pag.PerPage).Offset(pag.Offset).Find(&postList)

	return utils.Paginated(c, "success", postList, pag.BuildMeta(total))
}

// Show returns a single post
// GET /api/posts/:id
func (h *Handler) Show(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var post models.Post
	if err := db.Preload("Category").Preload("Author").Preload("Comments.User").
		Preload("KeywordsRel").
		Where("id = ? AND is_active = ?", id, true).
		First(&post).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}

	go db.Model(&post).UpdateColumn("views", gorm.Expr("views + 1"))

	return utils.Success(c, "success", post)
}

// IncrementView increments the post view count
// POST /api/posts/:id/increment-view
func (h *Handler) IncrementView(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	db.Model(&models.Post{}).Where("id = ?", id).
		UpdateColumn("views", gorm.Expr("views + 1"))

	return utils.Success(c, "success", nil)
}

// DashboardCreate creates a new post
// POST /api/dashboard/posts
func (h *Handler) DashboardCreate(c *fiber.Ctx) error {
	type CreateRequest struct {
		CategoryID      uint   `json:"category_id"`
		Title           string `json:"title" validate:"required,min=3,max=500"`
		Content         string `json:"content" validate:"required"`
		IsActive        bool   `json:"is_active"`
		IsFeatured      bool   `json:"is_featured"`
		Keywords        string `json:"keywords"`
		MetaDescription string `json:"meta_description" validate:"omitempty,max=500"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	countryCode, _ := c.Locals("country_code").(string)
	user, _ := c.Locals("user").(*models.User)

	slug := generateSlug(req.Title)
	post := models.Post{
		Title:    utils.SanitizeInput(req.Title),
		Content:  req.Content,
		Slug:     slug,
		IsActive: req.IsActive,
		IsFeatured: req.IsFeatured,
		Country:  countryCode,
	}
	if req.CategoryID > 0 {
		post.CategoryID = &req.CategoryID
	}
	if req.Keywords != "" {
		post.Keywords = &req.Keywords
	}
	if req.MetaDescription != "" {
		post.MetaDescription = &req.MetaDescription
	}
	if user != nil {
		post.AuthorID = &user.ID
	}

	// Handle image upload
	if img, err := c.FormFile("image"); err == nil {
		fileSvc := services.NewFileService()
		uploaded, err := fileSvc.UploadImage(img, "posts")
		if err == nil {
			post.Image = &uploaded.Path
		}
	}

	if err := db.Create(&post).Error; err != nil {
		return utils.InternalError(c, "فشل إنشاء المنشور")
	}

	return utils.Created(c, "تم إنشاء المنشور بنجاح", post)
}

// DashboardUpdate updates a post
// POST /api/dashboard/posts/:id
func (h *Handler) DashboardUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var post models.Post
	if err := db.First(&post, id).Error; err != nil {
		return utils.NotFound(c)
	}

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	db.Model(&post).Updates(updates)
	return utils.Success(c, "تم تحديث المنشور بنجاح", post)
}

// DashboardToggleStatus toggles post active status
// POST /api/dashboard/posts/:id/toggle-status
func (h *Handler) DashboardToggleStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var post models.Post
	if err := db.First(&post, id).Error; err != nil {
		return utils.NotFound(c)
	}

	db.Model(&post).Update("is_active", !post.IsActive)
	return utils.Success(c, "تم تحديث حالة المنشور", post)
}

// DashboardDelete deletes a post
// DELETE /api/dashboard/posts/:id
func (h *Handler) DashboardDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	db.Delete(&models.Post{}, id)

	return utils.Success(c, "تم حذف المنشور بنجاح", nil)
}

func generateSlug(title string) string {
	slug := ""
	for _, r := range title {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			slug += string(r)
		} else if r >= 'A' && r <= 'Z' {
			slug += string(r + 32)
		} else if r == ' ' || r == '-' {
			slug += "-"
		}
	}
	if slug == "" {
		slug = strconv.FormatInt(int64(len(title)), 36)
	}
	return slug
}
