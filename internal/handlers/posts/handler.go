package posts

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

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

	catID := c.Query("category_id")
	search := c.Query("search")
	featured := c.Query("featured")

	postList, total, err := h.svc.List(countryID, catID, search, featured, pag.PerPage, pag.Offset)
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
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
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

	var req services.UpdatePostRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	post, err := h.svc.Update(countryID, id, &req)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, "فشل تحديث المنشور")
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

	post, err := h.svc.GetByID(countryID, id)
	if err != nil {
		return utils.NotFound(c)
	}

	newStatus := !post.IsActive
	updatedPost, err := h.svc.Update(countryID, id, &services.UpdatePostRequest{
		IsActive: &newStatus,
	})
	if err != nil {
		return utils.InternalError(c, "فشل تحديث حالة المنشور")
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

	if err := h.svc.Delete(countryID, id); err != nil {
		return utils.InternalError(c, "فشل حذف المنشور")
	}

	return utils.Success(c, "تم حذف المنشور بنجاح", nil)
}
