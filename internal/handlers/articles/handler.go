package articles

import (
	"mime"
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

	status := 1
	filters := &models.ArticleFilter{
		Status: &status,
	}

	// Filters
	if gradeLevel := c.Query("grade_level"); gradeLevel != "" {
		filters.GradeLevel = gradeLevel
	}
	if subjectID := c.Query("subject_id"); subjectID != "" {
		if id, err := strconv.ParseUint(subjectID, 10, 64); err == nil {
			parsedID := uint(id)
			filters.SubjectID = &parsedID
		}
	}
	if semesterID := c.Query("semester_id"); semesterID != "" {
		if id, err := strconv.ParseUint(semesterID, 10, 64); err == nil {
			parsedID := uint(id)
			filters.SemesterID = &parsedID
		}
	}
	if q := c.Query("q"); q != "" {
		filters.Query = q
	} else if search := c.Query("search"); search != "" {
		filters.Query = search
	}
	if fc := c.Query("file_category"); fc != "" {
		filters.FileCategory = fc
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

// GetDownloadToken returns a short-lived signed token for a file download.
// GET /api/articles/file/:id/download-url
func (h *Handler) GetDownloadToken(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	token, err := h.svc.GetSignedDownloadToken(countryID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", fiber.Map{"token": token})
}

// DownloadFileSigned validates a signed token and serves the file.
// GET /api/articles/download?token=...
func (h *Handler) DownloadFileSigned(c *fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		return utils.BadRequest(c, "رمز التحميل مطلوب")
	}

	file, absPath, err := h.svc.GetFileBySignedToken(token)
	if err != nil {
		return utils.Unauthorized(c, "رمز التحميل غير صالح أو منتهي الصلاحية")
	}

	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": file.FileName})
	c.Set("Content-Disposition", disposition)
	c.Set("Content-Type", file.MimeType)
	return c.SendFile(absPath)
}

// DownloadFile serves an article file for download (legacy — prefer signed URL via GetDownloadToken)
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

	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": file.FileName})
	c.Set("Content-Disposition", disposition)
	c.Set("Content-Type", file.MimeType)
	return c.SendFile(absPath)
}

// ------- Dashboard Article Handlers -------

// DashboardList returns all articles for dashboard management
// GET /api/dashboard/articles
func (h *Handler) DashboardList(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	pag := utils.GetPagination(c)

	filters := &models.ArticleFilter{
		Order: "created_at DESC",
	}

	if status := c.Query("status"); status != "" {
		if s, err := strconv.Atoi(status); err == nil {
			filters.Status = &s
		}
	}
	if q := c.Query("q"); q != "" {
		filters.Query = q
	} else if search := c.Query("search"); search != "" {
		filters.Query = search
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
	var req services.ArticleInput
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	user := middleware.GetUser(c)
	countryID, _ := c.Locals("country_id").(database.CountryID)

	var authorID *uint
	if user != nil {
		authorID = &user.ID
	}

	article, err := h.svc.CreateArticle(countryID, &req, authorID)
	if err != nil {
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

	var req services.ArticleInput
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	user := middleware.GetUser(c)
	var authorID *uint
	if user != nil {
		authorID = &user.ID
	}

	article, err := h.svc.UpdateArticle(countryID, id, &req, authorID)
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
