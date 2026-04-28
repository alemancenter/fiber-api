package grades

import (
	"strconv"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

const (
	classesAndFilterTTL = time.Hour
)

// Handler handles school classes, subjects, semesters, and grade-based content
type Handler struct {
	svc     services.GradeService
	fileSvc *services.FileService
}

// New creates a new grades Handler
func New(svc services.GradeService, fileSvc *services.FileService) *Handler {
	return &Handler{
		svc:     svc,
		fileSvc: fileSvc,
	}
}

// ListSchoolClasses returns all school classes.
// Result is cached per country for 1 hour — this data changes only via admin dashboard.
// GET /api/school-classes
func (h *Handler) ListSchoolClasses(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)

	classes, err := h.svc.ListSchoolClasses(countryID)
	if err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "success", classes)
}

// GetSchoolClass returns a single school class with its subjects
// GET /api/school-classes/:id
func (h *Handler) GetSchoolClass(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	class, err := h.svc.GetSchoolClass(countryID, id)
	if err != nil {
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

	subjects, err := h.svc.ListSubjects(countryID, classID)
	if err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "success", subjects)
}

// ListSemesters returns semesters for a subject's grade level
// GET /api/filter/semesters/:subjectId
func (h *Handler) ListSemesters(c *fiber.Ctx) error {
	subjectID, err := strconv.ParseUint(c.Params("subjectId"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	semesters, subject, err := h.svc.ListSemesters(countryID, subjectID)
	if err != nil {
		if subject == nil {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", fiber.Map{
		"subject":   subject,
		"class_id":  subject.GradeLevel,
		"semesters": semesters,
	})
}

// FilterMeta returns top-level filter metadata (cached per country).
// GET /api/filter
func (h *Handler) FilterMeta(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)

	classes, err := h.svc.FilterMeta(countryID)
	if err != nil {
		return utils.InternalError(c)
	}
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
	pag := utils.GetPagination(c)

	articles, total, err := h.svc.ListGradeArticles(countryID, id, pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c)
	}

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

	file, err := h.fileSvc.FindByID(countryID, id)
	if err != nil {
		return utils.NotFound(c)
	}

	absPath := h.fileSvc.GetAbsPath(file.FilePath)

	// Increment view count
	go h.fileSvc.IncrementViewCount(countryID, id)

	c.Set("Content-Disposition", "attachment; filename=\""+file.FileName+"\"")
	c.Set("Content-Type", file.MimeType)
	return c.SendFile(absPath)
}

// ── Dashboard ────────────────────────────────────────────────────────────────

// DashboardListSchoolClasses returns all classes for dashboard
// GET /api/dashboard/school-classes
func (h *Handler) DashboardListSchoolClasses(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	pag := utils.GetPagination(c)

	classes, total, err := h.svc.ListSchoolClassesDashboard(countryID, pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", classes, pag.BuildMeta(total))
}

// DashboardCreateSchoolClass creates a school class
// POST /api/dashboard/school-classes
func (h *Handler) DashboardCreateSchoolClass(c *fiber.Ctx) error {
	type CreateRequest struct {
		GradeName  string `json:"grade_name" validate:"required,min=2,max=255"`
		GradeLevel int    `json:"grade_level"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}
	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	class := models.SchoolClass{GradeName: req.GradeName, GradeLevel: req.GradeLevel}
	if err := h.svc.CreateSchoolClass(countryID, &class); err != nil {
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

	type UpdateRequest struct {
		GradeName  string `json:"grade_name"`
		GradeLevel int    `json:"grade_level"`
	}
	var req UpdateRequest
	c.BodyParser(&req)

	updates := map[string]interface{}{}
	if req.GradeName != "" {
		updates["grade_name"] = req.GradeName
	}
	if req.GradeLevel > 0 {
		updates["grade_level"] = req.GradeLevel
	}

	class, err := h.svc.UpdateSchoolClass(countryID, id, updates)
	if err != nil {
		return utils.NotFound(c)
	}

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
	if err := h.svc.DeleteSchoolClass(countryID, id); err != nil {
		return utils.InternalError(c, "فشل الحذف")
	}

	return utils.Success(c, "تم حذف الصف بنجاح", nil)
}

// DashboardListSubjects returns all subjects for dashboard
func (h *Handler) DashboardListSubjects(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	pag := utils.GetPagination(c)

	subjects, total, err := h.svc.ListSubjectsDashboard(countryID, pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", subjects, pag.BuildMeta(total))
}

// DashboardCreateSubject creates a subject
func (h *Handler) DashboardCreateSubject(c *fiber.Ctx) error {
	type CreateRequest struct {
		SubjectName string `json:"subject_name" validate:"required,min=2,max=255"`
		GradeLevel  uint   `json:"grade_level" validate:"required"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}
	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	subject := models.Subject{SubjectName: req.SubjectName, GradeLevel: req.GradeLevel}
	if err := h.svc.CreateSubject(countryID, &subject); err != nil {
		return utils.InternalError(c, "فشل إنشاء المادة")
	}

	return utils.Created(c, "تم إنشاء المادة بنجاح", subject)
}

// DashboardListSemesters returns all semesters for dashboard
func (h *Handler) DashboardListSemesters(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	pag := utils.GetPagination(c)

	semesters, total, err := h.svc.ListSemestersDashboard(countryID, pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", semesters, pag.BuildMeta(total))
}

// DashboardCreateSemester creates a semester
func (h *Handler) DashboardCreateSemester(c *fiber.Ctx) error {
	type CreateRequest struct {
		SemesterName string `json:"semester_name" validate:"required,min=2,max=255"`
		GradeLevel   uint   `json:"grade_level"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	semester := models.Semester{SemesterName: req.SemesterName, GradeLevel: req.GradeLevel}
	if err := h.svc.CreateSemester(countryID, &semester); err != nil {
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

	type UpdateRequest struct {
		SemesterName string `json:"semester_name"`
		GradeLevel   uint   `json:"grade_level"`
	}
	var req UpdateRequest
	c.BodyParser(&req)

	updates := map[string]interface{}{}
	if req.SemesterName != "" {
		updates["semester_name"] = req.SemesterName
	}
	if req.GradeLevel > 0 {
		updates["grade_level"] = req.GradeLevel
	}

	semester, err := h.svc.UpdateSemester(countryID, id, updates)
	if err != nil {
		return utils.NotFound(c)
	}

	return utils.Success(c, "تم تحديث الفصل الدراسي بنجاح", semester)
}

// DashboardDeleteSemester deletes a semester
func (h *Handler) DashboardDeleteSemester(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	if err := h.svc.DeleteSemester(countryID, id); err != nil {
		return utils.InternalError(c, "فشل الحذف")
	}

	return utils.Success(c, "تم حذف الفصل الدراسي بنجاح", nil)
}
