package grades

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Handler handles school classes, subjects, semesters, and grade-based content
type Handler struct{}

// New creates a new grades Handler
func New() *Handler { return &Handler{} }

// ListSchoolClasses returns all school classes
// GET /api/school-classes
func (h *Handler) ListSchoolClasses(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var classes []models.SchoolClass
	db.Where("is_active = ?", true).Order("order ASC, name ASC").Find(&classes)
	return utils.Success(c, "success", classes)
}

// GetSchoolClass returns a single school class
// GET /api/school-classes/:id
func (h *Handler) GetSchoolClass(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var class models.SchoolClass
	if err := db.Preload("Subjects").First(&class, id).Error; err != nil {
		return utils.NotFound(c)
	}

	return utils.Success(c, "success", class)
}

// ListSubjects returns subjects for a class
// GET /api/filter/subjects/:classId
func (h *Handler) ListSubjects(c *fiber.Ctx) error {
	classID, err := strconv.ParseUint(c.Params("classId"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var subjects []models.Subject
	db.Where("school_class_id = ? AND is_active = ?", classID, true).
		Order("order ASC, name ASC").
		Find(&subjects)

	return utils.Success(c, "success", subjects)
}

// ListSemesters returns semesters for a subject
// GET /api/filter/semesters/:subjectId
func (h *Handler) ListSemesters(c *fiber.Ctx) error {
	subjectID, err := strconv.ParseUint(c.Params("subjectId"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var semesters []models.Semester
	db.Where("subject_id = ? AND is_active = ?", subjectID, true).
		Order("order ASC, name ASC").
		Find(&semesters)

	return utils.Success(c, "success", semesters)
}

// FilterMeta returns top-level filter metadata (classes, subjects, semesters)
// GET /api/filter
func (h *Handler) FilterMeta(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var classes []models.SchoolClass
	db.Where("is_active = ?", true).Order("order ASC").Find(&classes)

	return utils.Success(c, "success", fiber.Map{"classes": classes})
}

// GetGradeArticles returns articles for a specific grade
// GET /api/grades/articles/:id
func (h *Handler) GetGradeArticles(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	pag := utils.GetPagination(c)

	var articles []models.Article
	var total int64

	db.Model(&models.Article{}).
		Where("subject_id = ? AND status = ?", id, 1).
		Count(&total)
	db.Preload("Subject").Preload("Semester").Preload("Files").
		Where("subject_id = ? AND status = ?", id, 1).
		Order("published_at DESC").
		Limit(pag.PerPage).Offset(pag.Offset).
		Find(&articles)

	return utils.Paginated(c, "success", articles, pag.BuildMeta(total))
}

// DownloadGradeFile downloads a file for a grade article
// GET /api/grades/files/:id/download
func (h *Handler) DownloadGradeFile(c *fiber.Ctx) error {
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

	go db.Model(&file).UpdateColumn("view_count", gorm.Expr("view_count + 1"))

	c.Set("Content-Disposition", "attachment; filename=\""+file.FileName+"\"")
	c.Set("Content-Type", file.MimeType)
	return c.SendFile(absPath)
}

// DashboardListSchoolClasses returns all classes for dashboard
// GET /api/dashboard/school-classes
func (h *Handler) DashboardListSchoolClasses(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	pag := utils.GetPagination(c)

	var classes []models.SchoolClass
	var total int64

	db.Model(&models.SchoolClass{}).Count(&total)
	db.Order("order ASC, name ASC").Limit(pag.PerPage).Offset(pag.Offset).Find(&classes)

	return utils.Paginated(c, "success", classes, pag.BuildMeta(total))
}

// DashboardCreateSchoolClass creates a school class
// POST /api/dashboard/school-classes
func (h *Handler) DashboardCreateSchoolClass(c *fiber.Ctx) error {
	type CreateRequest struct {
		Name       string `json:"name" validate:"required,min=2,max=255"`
		GradeLevel string `json:"grade_level"`
		Order      int    `json:"order"`
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

	class := models.SchoolClass{
		Name:       req.Name,
		GradeLevel: req.GradeLevel,
		Order:      req.Order,
		IsActive:   true,
	}

	if err := db.Create(&class).Error; err != nil {
		return utils.InternalError(c, "فشل إنشاء الصف")
	}

	return utils.Created(c, "تم إنشاء الصف بنجاح", class)
}

// DashboardUpdateSchoolClass updates a school class
// PUT /api/dashboard/school-classes/:id
func (h *Handler) DashboardUpdateSchoolClass(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var class models.SchoolClass
	if err := db.First(&class, id).Error; err != nil {
		return utils.NotFound(c)
	}

	var updates map[string]interface{}
	c.BodyParser(&updates)
	db.Model(&class).Updates(updates)

	return utils.Success(c, "تم تحديث الصف بنجاح", class)
}

// DashboardDeleteSchoolClass deletes a school class
// DELETE /api/dashboard/school-classes/:id
func (h *Handler) DashboardDeleteSchoolClass(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	db.Delete(&models.SchoolClass{}, id)

	return utils.Success(c, "تم حذف الصف بنجاح", nil)
}

// DashboardListSubjects returns all subjects for dashboard
func (h *Handler) DashboardListSubjects(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	pag := utils.GetPagination(c)

	var subjects []models.Subject
	var total int64

	db.Model(&models.Subject{}).Count(&total)
	db.Preload("SchoolClass").Order("order ASC").Limit(pag.PerPage).Offset(pag.Offset).Find(&subjects)

	return utils.Paginated(c, "success", subjects, pag.BuildMeta(total))
}

// DashboardCreateSubject creates a subject
func (h *Handler) DashboardCreateSubject(c *fiber.Ctx) error {
	type CreateRequest struct {
		Name          string `json:"name" validate:"required,min=2,max=255"`
		SchoolClassID uint   `json:"school_class_id" validate:"required"`
		Order         int    `json:"order"`
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

	subject := models.Subject{
		Name:          req.Name,
		SchoolClassID: &req.SchoolClassID,
		Order:         req.Order,
		IsActive:      true,
	}

	if err := db.Create(&subject).Error; err != nil {
		return utils.InternalError(c, "فشل إنشاء المادة")
	}

	return utils.Created(c, "تم إنشاء المادة بنجاح", subject)
}

// DashboardListSemesters returns all semesters for dashboard
func (h *Handler) DashboardListSemesters(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	pag := utils.GetPagination(c)

	var semesters []models.Semester
	var total int64

	db.Model(&models.Semester{}).Count(&total)
	db.Preload("Subject").Order("order ASC").Limit(pag.PerPage).Offset(pag.Offset).Find(&semesters)

	return utils.Paginated(c, "success", semesters, pag.BuildMeta(total))
}

// DashboardCreateSemester creates a semester
func (h *Handler) DashboardCreateSemester(c *fiber.Ctx) error {
	type CreateRequest struct {
		Name      string `json:"name" validate:"required,min=2,max=255"`
		SubjectID uint   `json:"subject_id"`
		Order     int    `json:"order"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	semester := models.Semester{
		Name:     req.Name,
		Order:    req.Order,
		IsActive: true,
	}
	if req.SubjectID > 0 {
		semester.SubjectID = &req.SubjectID
	}

	if err := db.Create(&semester).Error; err != nil {
		return utils.InternalError(c, "فشل إنشاء الفصل الدراسي")
	}

	return utils.Created(c, "تم إنشاء الفصل الدراسي بنجاح", semester)
}

// DashboardUpdateSemester updates a semester
func (h *Handler) DashboardUpdateSemester(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var semester models.Semester
	if err := db.First(&semester, id).Error; err != nil {
		return utils.NotFound(c)
	}

	var updates map[string]interface{}
	c.BodyParser(&updates)
	db.Model(&semester).Updates(updates)

	return utils.Success(c, "تم تحديث الفصل الدراسي بنجاح", semester)
}

// DashboardDeleteSemester deletes a semester
func (h *Handler) DashboardDeleteSemester(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	db.Delete(&models.Semester{}, id)

	return utils.Success(c, "تم حذف الفصل الدراسي بنجاح", nil)
}
