package categories

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains categories route handlers
type Handler struct {
	svc services.CategoryService
}

// New creates a new categories Handler
func New(svc services.CategoryService) *Handler {
	return &Handler{svc: svc}
}

// List returns all active categories (cached per country).
// GET /api/categories
func (h *Handler) List(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)

	cats, err := h.svc.GetActiveCategories(countryID)
	if err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "success", cats)
}

// Show returns a single category
// GET /api/categories/:id
func (h *Handler) Show(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	category, err := h.svc.GetByID(countryID, id)
	if err != nil {
		if err == services.ErrNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", category)
}

// DashboardList returns all categories for dashboard management
// GET /api/dashboard/categories
func (h *Handler) DashboardList(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	pag := utils.GetPagination(c)

	search := c.Query("search")
	isActiveStr := c.Query("is_active")

	categoryList, total, err := h.svc.ListDashboard(countryID, search, isActiveStr, pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", categoryList, pag.BuildMeta(total))
}

// DashboardShow returns a single category (dashboard)
// GET /api/dashboard/categories/:id
func (h *Handler) DashboardShow(c *fiber.Ctx) error {
	return h.Show(c)
}

// DashboardCreate creates a new category
// POST /api/dashboard/categories
func (h *Handler) DashboardCreate(c *fiber.Ctx) error {
	var req services.CreateCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}
	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	category, err := h.svc.Create(countryID, &req)
	if err != nil {
		return utils.InternalError(c, "فشل إنشاء التصنيف")
	}

	return utils.Created(c, "تم إنشاء التصنيف بنجاح", category)
}

// DashboardUpdate updates a category
// POST /api/dashboard/categories/:id/update
func (h *Handler) DashboardUpdate(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	var req services.UpdateCategoryRequest
	c.BodyParser(&req)

	category, err := h.svc.Update(countryID, id, &req)
	if err != nil {
		if err == services.ErrNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, "فشل تحديث التصنيف")
	}

	return utils.Success(c, "تم تحديث التصنيف بنجاح", category)
}

// DashboardDelete deletes a category
// DELETE /api/dashboard/categories/:id
func (h *Handler) DashboardDelete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	if err := h.svc.Delete(countryID, id); err != nil {
		return utils.InternalError(c, "فشل حذف التصنيف")
	}

	return utils.Success(c, "تم حذف التصنيف بنجاح", nil)
}

// DashboardToggleStatus toggles category active status
// POST /api/dashboard/categories/:id/toggle
func (h *Handler) DashboardToggleStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	category, err := h.svc.GetByID(countryID, id)
	if err != nil {
		return utils.NotFound(c)
	}

	newStatus := !category.IsActive
	updatedCategory, err := h.svc.Update(countryID, id, &services.UpdateCategoryRequest{
		IsActive: &newStatus,
	})
	if err != nil {
		return utils.InternalError(c, "فشل تحديث حالة التصنيف")
	}

	return utils.Success(c, "تم تحديث حالة التصنيف", updatedCategory)
}
