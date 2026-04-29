package files

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains file management route handlers
type Handler struct {
	svc *services.FileService
}

// New creates a new files Handler
func New(svc *services.FileService) *Handler {
	return &Handler{svc: svc}
}

// Info returns file metadata with its parent article or post.
// GET /api/files/:id/info
func (h *Handler) Info(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	resp, err := h.svc.GetFileWithParent(countryID, id)
	if err != nil {
		if err == services.ErrNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", resp)
}

// IncrementView increments the file view count
// POST /api/files/:id/increment-view
func (h *Handler) IncrementView(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	if err := h.svc.IncrementViewCount(countryID, id); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", nil)
}

// UploadImage handles public image upload
// POST /api/upload/image
func (h *Handler) UploadImage(c *fiber.Ctx) error {
	photo, err := c.FormFile("image")
	if err != nil {
		return utils.BadRequest(c, "الصورة مطلوبة")
	}

	uploaded, err := h.svc.UploadImage(photo, "images")
	if err != nil {
		return utils.BadRequest(c, err.Error())
	}

	return utils.Created(c, "تم رفع الصورة بنجاح", services.UploadResponse{
		Path: uploaded.Path,
		URL:  uploaded.URL,
		Name: uploaded.Name,
		Size: uploaded.Size,
		Type: uploaded.MimeType,
	})
}

// UploadDocument handles public document upload
// POST /api/upload/file
func (h *Handler) UploadDocument(c *fiber.Ctx) error {
	doc, err := c.FormFile("file")
	if err != nil {
		return utils.BadRequest(c, "الملف مطلوب")
	}

	uploaded, err := h.svc.UploadDocument(doc, "documents")
	if err != nil {
		return utils.BadRequest(c, err.Error())
	}

	return utils.Created(c, "تم رفع الملف بنجاح", services.UploadResponse{
		Path: uploaded.Path,
		URL:  uploaded.URL,
		Name: uploaded.Name,
		Size: uploaded.Size,
		Type: uploaded.MimeType,
	})
}

// DashboardList returns all files for dashboard management
// GET /api/dashboard/files
func (h *Handler) DashboardList(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	pag := utils.GetPagination(c)

	fileType := c.Query("type")
	articleID := c.Query("article_id")

	fileList, total, err := h.svc.List(countryID, fileType, articleID, pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", fileList, pag.BuildMeta(total))
}

// DashboardShow returns a single file (flat, no parent join)
// GET /api/dashboard/files/:id
func (h *Handler) DashboardShow(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}
	countryID, _ := c.Locals("country_id").(database.CountryID)
	file, err := h.svc.FindByID(countryID, id)
	if err != nil {
		if err == services.ErrNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}
	return utils.Success(c, "success", file)
}

// DashboardUpload uploads a file and creates a record
// POST /api/dashboard/files
func (h *Handler) DashboardUpload(c *fiber.Ctx) error {
	uploadedFile, err := c.FormFile("file")
	if err != nil {
		return utils.BadRequest(c, "الملف مطلوب")
	}

	var uploaded *services.UploadedFile

	// Try as document first, fallback to image
	uploaded, err = h.svc.UploadDocument(uploadedFile, "uploads")
	if err != nil {
		uploaded, err = h.svc.UploadImage(uploadedFile, "uploads")
		if err != nil {
			return utils.BadRequest(c, err.Error())
		}
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	var articleIDPtr *uint
	if articleID := c.FormValue("article_id"); articleID != "" {
		if id, err := strconv.ParseUint(articleID, 10, 64); err == nil {
			uid := uint(id)
			articleIDPtr = &uid
		}
	}

	file, err := h.svc.CreateRecord(countryID, uploaded, articleIDPtr)
	if err != nil {
		return utils.InternalError(c, "فشل حفظ بيانات الملف")
	}

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

	var req services.UpdateFileInput
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	file, err := h.svc.UpdateRecord(countryID, id, &req)
	if err != nil {
		if err == services.ErrNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, "فشل تحديث الملف")
	}

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

	if err := h.svc.DeleteRecord(countryID, id); err != nil {
		if err == services.ErrNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, "فشل حذف الملف")
	}

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

	file, err := h.svc.FindByID(countryID, id)
	if err != nil {
		return utils.NotFound(c)
	}

	// Use SafeGetAbsPath to reject any DB-stored path that escapes the storage root
	absPath, err := h.svc.SafeGetAbsPath(file.FilePath)
	if err != nil {
		return utils.InternalError(c, "مسار الملف غير صالح")
	}

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
	relPath := c.Query("path")
	if relPath == "" {
		return utils.BadRequest(c, "مسار الملف مطلوب")
	}

	absPath, err := h.svc.SafeGetAbsPath(relPath)
	if err != nil {
		return utils.BadRequest(c, "مسار الملف غير صالح")
	}
	return c.SendFile(absPath)
}
