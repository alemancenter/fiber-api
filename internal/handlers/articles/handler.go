package articles

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/middleware"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Handler contains articles route handlers
type Handler struct {
	svc services.ArticleService
}

// New creates a new articles Handler
func New(svc services.ArticleService) *Handler {
	return &Handler{
		svc: svc,
	}
}

// List returns a paginated list of published articles
// GET /api/articles
func (h *Handler) List(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	pag := utils.GetPagination(c)

	filters := map[string]interface{}{
		"status": 1,
	}

	// Filters
	if gradeLevel := c.Query("grade_level"); gradeLevel != "" {
		filters["grade_level"] = gradeLevel
	}
	if subjectID := c.Query("subject_id"); subjectID != "" {
		filters["subject_id"] = subjectID
	}
	if semesterID := c.Query("semester_id"); semesterID != "" {
		filters["semester_id"] = semesterID
	}
	if q := c.Query("q"); q != "" {
		filters["q"] = q
	} else if search := c.Query("search"); search != "" {
		filters["q"] = search
	}

	articles, total, err := h.svc.List(countryID, pag, filters)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", articles, pag.BuildMeta(total))
}

// Show returns a single article
// GET /api/articles/:id
func (h *Handler) Show(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	article, err := h.svc.GetByID(countryID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}

	if article.Status != 1 {
		return utils.NotFound(c)
	}

	return utils.Success(c, "success", article)
}

// ByClass returns articles filtered by grade level
// GET /api/articles/by-class/:grade_level
func (h *Handler) ByClass(c *fiber.Ctx) error {
	gradeLevel := c.Params("grade_level")
	countryID, _ := c.Locals("country_id").(database.CountryID)
	pag := utils.GetPagination(c)

	articles, total, err := h.svc.GetByGradeLevel(countryID, gradeLevel, pag)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", articles, pag.BuildMeta(total))
}

// ByKeyword returns articles filtered by keyword
// GET /api/articles/by-keyword/:keyword
func (h *Handler) ByKeyword(c *fiber.Ctx) error {
	keyword := c.Params("keyword")
	countryID, _ := c.Locals("country_id").(database.CountryID)
	pag := utils.GetPagination(c)

	articles, total, err := h.svc.GetByKeyword(countryID, keyword, pag)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Paginated(c, "success", []models.Article{}, pag.BuildMeta(0))
		}
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", articles, pag.BuildMeta(total))
}

// DownloadFile serves an article file for download
// GET /api/articles/file/:id/download
func (h *Handler) DownloadFile(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	file, absPath, err := h.svc.GetFileForDownload(countryID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}

	c.Set("Content-Disposition", "attachment; filename=\""+file.FileName+"\"")
	c.Set("Content-Type", file.MimeType)
	return c.SendFile(absPath)
}

// ------- Dashboard Article Handlers -------

// DashboardList returns all articles for dashboard management
// GET /api/dashboard/articles
func (h *Handler) DashboardList(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	pag := utils.GetPagination(c)

	filters := map[string]interface{}{}
	filters["order"] = "created_at DESC"

	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if q := c.Query("q"); q != "" {
		filters["q"] = q
	} else if search := c.Query("search"); search != "" {
		filters["q"] = search
	}

	articles, total, err := h.svc.List(countryID, pag, filters)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", articles, pag.BuildMeta(total))
}

// DashboardCreateData returns metadata needed by the create article form.
// GET /api/dashboard/articles/create
func (h *Handler) DashboardCreateData(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)

	data, err := h.svc.GetDashboardCreateData(countryID)
	if err != nil {
		return utils.InternalError(c, "فشل تحميل بيانات إنشاء المقالة")
	}

	return utils.Success(c, "success", data)
}

// DashboardEditData returns the article with auxiliary lists for the edit form.
// GET /api/dashboard/articles/:id/edit
func (h *Handler) DashboardEditData(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	data, err := h.svc.GetDashboardEditData(countryID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, "فشل تحميل بيانات المقالة")
	}

	return utils.Success(c, "success", data)
}

// DashboardCreate creates a new article
// POST /api/dashboard/articles
func (h *Handler) DashboardCreate(c *fiber.Ctx) error {
	type CreateRequest struct {
		Title           string `json:"title" validate:"required,min=3,max=500"`
		Content         string `json:"content" validate:"required"`
		GradeLevel      string `json:"grade_level"`
		SubjectID       uint   `json:"subject_id"`
		SemesterID      uint   `json:"semester_id"`
		MetaDescription string `json:"meta_description" validate:"omitempty,max=500"`
		Keywords        string `json:"keywords"`
		Status          int8   `json:"status" validate:"oneof=0 1"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	user := middleware.GetUser(c)
	countryID, _ := c.Locals("country_id").(database.CountryID)

	article := models.Article{
		Title:   utils.SanitizeInput(req.Title),
		Content: req.Content,
		Status:  req.Status,
	}

	if req.GradeLevel != "" {
		article.GradeLevel = &req.GradeLevel
	}
	if req.SubjectID > 0 {
		article.SubjectID = &req.SubjectID
	}
	if req.SemesterID > 0 {
		article.SemesterID = &req.SemesterID
	}
	if req.MetaDescription != "" {
		article.MetaDescription = &req.MetaDescription
	}
	if req.Keywords != "" {
		article.Keywords = &req.Keywords
	}

	var authorID *uint
	if user != nil {
		authorID = &user.ID
		article.AuthorID = authorID
	}

	if err := h.svc.CreateArticle(countryID, &article, authorID); err != nil {
		return utils.InternalError(c, "فشل إنشاء المقالة")
	}

	return utils.Created(c, "تم إنشاء المقالة بنجاح", article)
}

// DashboardUpdate updates an existing article
// PUT /api/dashboard/articles/:id
func (h *Handler) DashboardUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	type UpdateRequest struct {
		Title           string `json:"title"`
		Content         string `json:"content"`
		GradeLevel      string `json:"grade_level"`
		SubjectID       *uint  `json:"subject_id"`
		SemesterID      *uint  `json:"semester_id"`
		MetaDescription string `json:"meta_description"`
		Keywords        string `json:"keywords"`
		Status          *int8  `json:"status"`
	}
	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = utils.SanitizeInput(req.Title)
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.GradeLevel != "" {
		updates["grade_level"] = req.GradeLevel
	}
	if req.SubjectID != nil {
		updates["subject_id"] = req.SubjectID
	}
	if req.SemesterID != nil {
		updates["semester_id"] = req.SemesterID
	}
	if req.MetaDescription != "" {
		updates["meta_description"] = req.MetaDescription
	}
	if req.Keywords != "" {
		updates["keywords"] = req.Keywords
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	user := middleware.GetUser(c)
	var authorID *uint
	if user != nil {
		authorID = &user.ID
	}

	article, err := h.svc.UpdateArticle(countryID, id, updates, authorID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, "فشل تحديث المقالة")
	}

	return utils.Success(c, "تم تحديث المقالة بنجاح", article)
}

// DashboardDelete deletes an article
// DELETE /api/dashboard/articles/:id
func (h *Handler) DashboardDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	user := middleware.GetUser(c)
	var authorID *uint
	if user != nil {
		authorID = &user.ID
	}

	if err := h.svc.DeleteArticle(countryID, id, authorID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, "فشل حذف المقالة")
	}

	return utils.Success(c, "تم حذف المقالة بنجاح", nil)
}

// DashboardPublish publishes an article
// POST /api/dashboard/articles/:id/publish
func (h *Handler) DashboardPublish(c *fiber.Ctx) error {
	return h.setArticleStatus(c, 1, "تم نشر المقالة بنجاح")
}

// DashboardUnpublish unpublishes an article
// POST /api/dashboard/articles/:id/unpublish
func (h *Handler) DashboardUnpublish(c *fiber.Ctx) error {
	return h.setArticleStatus(c, 0, "تم إلغاء نشر المقالة بنجاح")
}

func (h *Handler) setArticleStatus(c *fiber.Ctx, status int8, message string) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	article, err := h.svc.SetArticleStatus(countryID, id, status)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}

	return utils.Success(c, message, article)
}

// DashboardStats returns article statistics
// GET /api/dashboard/articles/stats
func (h *Handler) DashboardStats(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)

	stats, err := h.svc.GetDashboardStats(countryID)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", stats)
}
