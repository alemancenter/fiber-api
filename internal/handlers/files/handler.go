package files

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Handler contains file management route handlers
type Handler struct{}

// New creates a new files Handler
func New() *Handler { return &Handler{} }

// Info returns file metadata
// GET /api/files/:id/info
func (h *Handler) Info(c *fiber.Ctx) error {
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

	return utils.Success(c, "success", file)
}

// IncrementView increments the file view count
// POST /api/files/:id/increment-view
func (h *Handler) IncrementView(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	db.Model(&models.File{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1"))

	return utils.Success(c, "success", nil)
}

// UploadImage handles public image upload
// POST /api/upload/image
func (h *Handler) UploadImage(c *fiber.Ctx) error {
	photo, err := c.FormFile("image")
	if err != nil {
		return utils.BadRequest(c, "الصورة مطلوبة")
	}

	fileSvc := services.NewFileService()
	uploaded, err := fileSvc.UploadImage(photo, "images")
	if err != nil {
		return utils.BadRequest(c, err.Error())
	}

	return utils.Created(c, "تم رفع الصورة بنجاح", fiber.Map{
		"path": uploaded.Path,
		"url":  uploaded.URL,
		"name": uploaded.Name,
		"size": uploaded.Size,
		"type": uploaded.MimeType,
	})
}

// UploadDocument handles public document upload
// POST /api/upload/file
func (h *Handler) UploadDocument(c *fiber.Ctx) error {
	doc, err := c.FormFile("file")
	if err != nil {
		return utils.BadRequest(c, "الملف مطلوب")
	}

	fileSvc := services.NewFileService()
	uploaded, err := fileSvc.UploadDocument(doc, "documents")
	if err != nil {
		return utils.BadRequest(c, err.Error())
	}

	return utils.Created(c, "تم رفع الملف بنجاح", fiber.Map{
		"path": uploaded.Path,
		"url":  uploaded.URL,
		"name": uploaded.Name,
		"size": uploaded.Size,
		"type": uploaded.MimeType,
	})
}

// DashboardList returns all files for dashboard management
// GET /api/dashboard/files
func (h *Handler) DashboardList(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	pag := utils.GetPagination(c)

	var fileList []models.File
	var total int64

	query := db.Model(&models.File{})
	if fileType := c.Query("type"); fileType != "" {
		query = query.Where("file_type = ?", fileType)
	}
	if articleID := c.Query("article_id"); articleID != "" {
		query = query.Where("article_id = ?", articleID)
	}

	query.Count(&total)
	query.Order("created_at DESC").Limit(pag.PerPage).Offset(pag.Offset).Find(&fileList)

	return utils.Paginated(c, "success", fileList, pag.BuildMeta(total))
}

// DashboardShow returns a single file
// GET /api/dashboard/files/:id
func (h *Handler) DashboardShow(c *fiber.Ctx) error {
	return h.Info(c)
}

// DashboardUpload uploads a file and creates a record
// POST /api/dashboard/files
func (h *Handler) DashboardUpload(c *fiber.Ctx) error {
	uploadedFile, err := c.FormFile("file")
	if err != nil {
		return utils.BadRequest(c, "الملف مطلوب")
	}

	fileSvc := services.NewFileService()
	var uploaded *services.UploadedFile

	// Try as document first, fallback to image
	uploaded, err = fileSvc.UploadDocument(uploadedFile, "uploads")
	if err != nil {
		uploaded, err = fileSvc.UploadImage(uploadedFile, "uploads")
		if err != nil {
			return utils.BadRequest(c, err.Error())
		}
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	file := models.File{
		FilePath: uploaded.Path,
		FileType: uploaded.Ext,
		FileName: uploaded.Name,
		FileSize: uploaded.Size,
		MimeType: uploaded.MimeType,
	}

	if articleID := c.FormValue("article_id"); articleID != "" {
		if id, err := strconv.ParseUint(articleID, 10, 64); err == nil {
			uid := uint(id)
			file.ArticleID = &uid
		}
	}

	db.Create(&file)
	return utils.Created(c, "تم رفع الملف بنجاح", file)
}

// DashboardUpdate updates file metadata
// PUT /api/dashboard/files/:id
func (h *Handler) DashboardUpdate(c *fiber.Ctx) error {
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

	var updates map[string]interface{}
	c.BodyParser(&updates)
	db.Model(&file).Updates(updates)

	return utils.Success(c, "تم تحديث الملف بنجاح", file)
}

// DashboardDelete deletes a file
// DELETE /api/dashboard/files/:id
func (h *Handler) DashboardDelete(c *fiber.Ctx) error {
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

	// Delete physical file
	fileSvc := services.NewFileService()
	fileSvc.Delete(file.FilePath)

	db.Delete(&file)
	return utils.Success(c, "تم حذف الملف بنجاح", nil)
}

// DashboardDownload serves a file for download
// GET /api/dashboard/files/:id/download
func (h *Handler) DashboardDownload(c *fiber.Ctx) error {
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

	c.Set("Content-Disposition", "attachment; filename=\""+file.FileName+"\"")
	c.Set("Content-Type", file.MimeType)
	return c.SendFile(absPath)
}

// SecureUploadImage uploads an image securely (with extra validation)
// POST /api/dashboard/secure/upload-image
func (h *Handler) SecureUploadImage(c *fiber.Ctx) error {
	return h.UploadImage(c)
}

// SecureUploadDocument uploads a document securely
// POST /api/dashboard/secure/upload-document
func (h *Handler) SecureUploadDocument(c *fiber.Ctx) error {
	return h.UploadDocument(c)
}

// SecureView serves a file securely
// GET /api/secure/view
func (h *Handler) SecureView(c *fiber.Ctx) error {
	path := c.Query("path")
	if path == "" {
		return utils.BadRequest(c, "مسار الملف مطلوب")
	}

	fileSvc := services.NewFileService()
	absPath := fileSvc.GetAbsPath(path)
	return c.SendFile(absPath)
}
