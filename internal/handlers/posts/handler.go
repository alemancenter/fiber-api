package posts

import (
	"strconv"
	"strings"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/alemancenter/fiber-api/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

var adminRoleNames = []string{"admin", "super_admin", "super-admin", "manager", "administrator", "root"}

func isAdminUser(user *models.User) bool {
	if user == nil {
		return false
	}
	if user.ID == 1 {
		return true
	}
	for _, role := range user.Roles {
		for _, name := range adminRoleNames {
			if strings.EqualFold(role.Name, name) {
				return true
			}
		}
	}
	return false
}

// Handler contains posts route handlers
type Handler struct {
	svc services.PostService
}

// New creates a new posts Handler
func New(svc services.PostService) *Handler {
	return &Handler{svc: svc}
}

// List returns a paginated list of published posts
// GET /api/posts
func (h *Handler) List(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	pag := utils.GetPagination(c)

	filter := &models.PostFilter{
		CategoryID: c.Query("category_id"),
		Search:     c.Query("search"),
		Featured:   c.Query("featured"),
	}

	postList, total, err := h.svc.List(countryID, filter, pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c)
	}

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

	post, err := h.svc.GetByID(countryID, id)
	if err != nil {
		if err == services.ErrNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}

	if !post.IsActive {
		return utils.NotFound(c)
	}

	go h.svc.IncrementView(countryID, uint64(post.ID))

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

	if err := h.svc.IncrementView(countryID, id); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", nil)
}

// DashboardCreate creates a new post
// POST /api/dashboard/posts
func (h *Handler) DashboardCreate(c *fiber.Ctx) error {
	var req services.CreatePostRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	countryCode, _ := c.Locals("country_code").(string)
	user, _ := c.Locals("user").(*models.User)

	var userID *uint
	if user != nil {
		userID = &user.ID
	}

	// Handle image upload
	var imagePath string
	if img, err := c.FormFile("image"); err == nil {
		fileRepo := repositories.NewFileRepository()
		fileSvc := services.NewFileService(fileRepo)
		uploaded, err := fileSvc.UploadImage(img, "posts")
		if err == nil {
			imagePath = uploaded.Path
		}
	}

	post, err := h.svc.Create(countryID, countryCode, userID, &req, imagePath)
	if err != nil {
		return utils.InternalError(c, "فشل إنشاء المنشور")
	}

	// Handle attachments
	if form, err := c.MultipartForm(); err == nil {
		if files := form.File["attachments[]"]; len(files) > 0 {
			fileRepo := repositories.NewFileRepository()
			fileSvc := services.NewFileService(fileRepo)
			postID := post.ID
			for _, file := range files {
				uploaded, uploadErr := fileSvc.UploadDocument(file, "posts/attachments")
				if uploadErr != nil {
					uploaded, uploadErr = fileSvc.UploadImage(file, "posts/attachments")
				}
				if uploadErr != nil {
					logger.Warn("post attachment upload failed",
						zap.String("filename", file.Filename),
						zap.Error(uploadErr),
					)
					continue
				}
				if _, recErr := fileSvc.CreateRecord(countryID, uploaded, nil, &postID, nil, nil); recErr != nil {
					logger.Warn("post attachment record creation failed",
						zap.String("path", uploaded.Path),
						zap.Error(recErr),
					)
				}
			}
		}
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
	caller, _ := c.Locals("user").(*models.User)

	var req services.UpdatePostRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	callerID := uint(0)
	if caller != nil {
		callerID = caller.ID
	}

	// Handle new_image upload if present
	if img, err := c.FormFile("new_image"); err == nil {
		fileRepo := repositories.NewFileRepository()
		fileSvc := services.NewFileService(fileRepo)
		uploaded, err := fileSvc.UploadImage(img, "posts")
		if err == nil {
			req.ImagePath = &uploaded.Path
		}
	}

	post, err := h.svc.Update(countryID, id, &req, callerID, isAdminUser(caller))
	if err != nil {
		switch err {
		case services.ErrNotFound:
			return utils.NotFound(c)
		case services.ErrForbidden:
			return utils.Forbidden(c)
		default:
			return utils.InternalError(c, "فشل تحديث المنشور")
		}
	}

	// Handle attachments
	if form, err := c.MultipartForm(); err == nil {
		if files := form.File["attachments[]"]; len(files) > 0 {
			fileRepo := repositories.NewFileRepository()
			fileSvc := services.NewFileService(fileRepo)
			postID := post.ID
			for _, file := range files {
				uploaded, uploadErr := fileSvc.UploadDocument(file, "posts/attachments")
				if uploadErr != nil {
					uploaded, uploadErr = fileSvc.UploadImage(file, "posts/attachments")
				}
				if uploadErr != nil {
					logger.Warn("post attachment upload failed",
						zap.String("filename", file.Filename),
						zap.Error(uploadErr),
					)
					continue
				}
				if _, recErr := fileSvc.CreateRecord(countryID, uploaded, nil, &postID, nil, nil); recErr != nil {
					logger.Warn("post attachment record creation failed",
						zap.String("path", uploaded.Path),
						zap.Error(recErr),
					)
				}
			}
		}
	}

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
	caller, _ := c.Locals("user").(*models.User)

	post, err := h.svc.GetByID(countryID, id)
	if err != nil {
		return utils.NotFound(c)
	}

	callerID := uint(0)
	if caller != nil {
		callerID = caller.ID
	}

	newStatus := !post.IsActive
	updatedPost, err := h.svc.Update(countryID, id, &services.UpdatePostRequest{
		IsActive: &newStatus,
	}, callerID, isAdminUser(caller))
	if err != nil {
		switch err {
		case services.ErrForbidden:
			return utils.Forbidden(c)
		default:
			return utils.InternalError(c, "فشل تحديث حالة المنشور")
		}
	}

	return utils.Success(c, "تم تحديث حالة المنشور", updatedPost)
}

// DashboardDelete deletes a post
// DELETE /api/dashboard/posts/:id
func (h *Handler) DashboardDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	caller, _ := c.Locals("user").(*models.User)

	callerID := uint(0)
	if caller != nil {
		callerID = caller.ID
	}

	if err := h.svc.Delete(countryID, id, callerID, isAdminUser(caller)); err != nil {
		switch err {
		case services.ErrNotFound:
			return utils.NotFound(c)
		case services.ErrForbidden:
			return utils.Forbidden(c)
		default:
			return utils.InternalError(c, "فشل حذف المنشور")
		}
	}

	return utils.Success(c, "تم حذف المنشور بنجاح", nil)
}
