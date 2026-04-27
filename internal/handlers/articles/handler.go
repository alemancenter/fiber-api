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
type Handler struct{}

// New creates a new articles Handler
func New() *Handler { return &Handler{} }

// List returns a paginated list of published articles
// GET /api/articles
func (h *Handler) List(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	pag := utils.GetPagination(c)

	var articles []models.Article
	var total int64

	query := db.Model(&models.Article{}).
		Preload("Subject").
		Preload("Semester").
		Preload("Files").
		Where("status = ?", 1)

	// Filters
	if gradeLevel := c.Query("grade_level"); gradeLevel != "" {
		query = query.Where("grade_level = ?", gradeLevel)
	}
	if subjectID := c.Query("subject_id"); subjectID != "" {
		query = query.Where("subject_id = ?", subjectID)
	}
	if semesterID := c.Query("semester_id"); semesterID != "" {
		query = query.Where("semester_id = ?", semesterID)
	}
	if search := c.Query("search"); search != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	query.Count(&total)
	query.Order("published_at DESC, created_at DESC").
		Limit(pag.PerPage).
		Offset(pag.Offset).
		Find(&articles)

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
	db := database.DBForCountry(countryID)

	var article models.Article
	if err := db.
		Preload("Subject").
		Preload("Semester").
		Preload("Files").
		Preload("Comments.User").
		Preload("KeywordsRel").
		Where("id = ? AND status = ?", id, 1).
		First(&article).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}

	// Increment view count (async)
	go db.Model(&article).UpdateColumn("visit_count", gorm.Expr("visit_count + 1"))

	return utils.Success(c, "success", article)
}

// ByClass returns articles filtered by grade level
// GET /api/articles/by-class/:grade_level
func (h *Handler) ByClass(c *fiber.Ctx) error {
	gradeLevel := c.Params("grade_level")
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	pag := utils.GetPagination(c)

	var articles []models.Article
	var total int64

	db.Model(&models.Article{}).Where("grade_level = ? AND status = ?", gradeLevel, 1).Count(&total)
	db.Preload("Subject").Preload("Semester").
		Where("grade_level = ? AND status = ?", gradeLevel, 1).
		Order("published_at DESC").
		Limit(pag.PerPage).Offset(pag.Offset).
		Find(&articles)

	return utils.Paginated(c, "success", articles, pag.BuildMeta(total))
}

// ByKeyword returns articles filtered by keyword
// GET /api/articles/by-keyword/:keyword
func (h *Handler) ByKeyword(c *fiber.Ctx) error {
	keyword := c.Params("keyword")
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	pag := utils.GetPagination(c)

	var articles []models.Article
	var total int64

	// Find keyword ID first
	var kw models.Keyword
	if err := db.Where("name = ? OR slug = ?", keyword, keyword).First(&kw).Error; err != nil {
		return utils.Paginated(c, "success", []models.Article{}, pag.BuildMeta(0))
	}

	subQuery := db.Table("article_keyword").Select("article_id").Where("keyword_id = ?", kw.ID)
	query := db.Model(&models.Article{}).Where("id IN (?) AND status = ?", subQuery, 1)

	query.Count(&total)
	query.Preload("Subject").Preload("Semester").
		Order("published_at DESC").
		Limit(pag.PerPage).Offset(pag.Offset).
		Find(&articles)

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
	db := database.DBForCountry(countryID)

	var file models.File
	if err := db.First(&file, id).Error; err != nil {
		return utils.NotFound(c)
	}

	fileSvc := services.NewFileService()
	absPath := fileSvc.GetAbsPath(file.FilePath)

	// Increment view count
	go db.Model(&file).UpdateColumn("view_count", gorm.Expr("view_count + 1"))

	c.Set("Content-Disposition", "attachment; filename=\""+file.FileName+"\"")
	c.Set("Content-Type", file.MimeType)
	return c.SendFile(absPath)
}

// ------- Dashboard Article Handlers -------

// DashboardList returns all articles for dashboard management
// GET /api/dashboard/articles
func (h *Handler) DashboardList(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	pag := utils.GetPagination(c)

	var articles []models.Article
	var total int64

	query := db.Model(&models.Article{}).Preload("Subject").Preload("Semester")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if search := c.Query("search"); search != "" {
		query = query.Where("title LIKE ?", "%"+search+"%")
	}

	query.Count(&total)
	query.Order("created_at DESC").Limit(pag.PerPage).Offset(pag.Offset).Find(&articles)

	return utils.Paginated(c, "success", articles, pag.BuildMeta(total))
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
	db := database.DBForCountry(countryID)

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
	if user != nil {
		article.AuthorID = &user.ID
	}

	if err := db.Create(&article).Error; err != nil {
		return utils.InternalError(c, "فشل إنشاء المقالة")
	}

	if user != nil {
		services.LogActivity("أنشأ مقالة: "+article.Title, "Article", article.ID, user.ID)
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
	db := database.DBForCountry(countryID)

	var article models.Article
	if err := db.First(&article, id).Error; err != nil {
		return utils.NotFound(c)
	}

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if err := db.Model(&article).Updates(updates).Error; err != nil {
		return utils.InternalError(c, "فشل تحديث المقالة")
	}

	user := middleware.GetUser(c)
	if user != nil {
		services.LogActivity("حدّث مقالة: "+article.Title, "Article", article.ID, user.ID)
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
	db := database.DBForCountry(countryID)

	var article models.Article
	if err := db.First(&article, id).Error; err != nil {
		return utils.NotFound(c)
	}

	if err := db.Delete(&article).Error; err != nil {
		return utils.InternalError(c, "فشل حذف المقالة")
	}

	user := middleware.GetUser(c)
	if user != nil {
		services.LogActivity("حذف مقالة: "+article.Title, "Article", article.ID, user.ID)
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
	db := database.DBForCountry(countryID)

	var article models.Article
	if err := db.First(&article, id).Error; err != nil {
		return utils.NotFound(c)
	}

	db.Model(&article).Update("status", status)
	return utils.Success(c, message, article)
}

// DashboardStats returns article statistics
// GET /api/dashboard/articles/stats
func (h *Handler) DashboardStats(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var total, published, draft int64
	db.Model(&models.Article{}).Count(&total)
	db.Model(&models.Article{}).Where("status = ?", 1).Count(&published)
	db.Model(&models.Article{}).Where("status = ?", 0).Count(&draft)

	return utils.Success(c, "success", fiber.Map{
		"total":     total,
		"published": published,
		"draft":     draft,
	})
}
