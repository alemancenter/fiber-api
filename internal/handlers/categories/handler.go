package categories

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Handler contains categories route handlers
type Handler struct{}

// New creates a new categories Handler
func New() *Handler { return &Handler{} }

// List returns all active categories
// GET /api/categories
func (h *Handler) List(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var categoryList []models.Category
	db.Where("is_active = ?", true).Order("order ASC, name ASC").Find(&categoryList)

	return utils.Success(c, "success", categoryList)
}

// Show returns a single category
// GET /api/categories/:id
func (h *Handler) Show(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var category models.Category
	if err := db.First(&category, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
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
	db := database.DBForCountry(countryID)
	pag := utils.GetPagination(c)

	var categoryList []models.Category
	var total int64

	db.Model(&models.Category{}).Count(&total)
	db.Order("order ASC, name ASC").Limit(pag.PerPage).Offset(pag.Offset).Find(&categoryList)

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
	type CreateRequest struct {
		Name        string `json:"name" validate:"required,min=2,max=255"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		Order       int    `json:"order"`
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

	slug := req.Slug
	if slug == "" {
		slug = generateCategorySlug(req.Name)
	}

	category := models.Category{
		Name:     req.Name,
		Slug:     slug,
		IsActive: true,
		Order:    req.Order,
	}
	if req.Description != "" {
		category.Description = &req.Description
	}

	if err := db.Create(&category).Error; err != nil {
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
	db := database.DBForCountry(countryID)

	var category models.Category
	if err := db.First(&category, id).Error; err != nil {
		return utils.NotFound(c)
	}

	var updates map[string]interface{}
	c.BodyParser(&updates)
	db.Model(&category).Updates(updates)

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
	db := database.DBForCountry(countryID)
	db.Delete(&models.Category{}, id)

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
	db := database.DBForCountry(countryID)

	var category models.Category
	if err := db.First(&category, id).Error; err != nil {
		return utils.NotFound(c)
	}

	db.Model(&category).Update("is_active", !category.IsActive)
	return utils.Success(c, "تم تحديث حالة التصنيف", category)
}

func generateCategorySlug(name string) string {
	slug := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			slug += string(r)
		} else if r >= 'A' && r <= 'Z' {
			slug += string(r + 32)
		} else if r == ' ' || r == '-' {
			slug += "-"
		}
	}
	return slug
}
