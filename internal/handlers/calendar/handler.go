package calendar

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains calendar route handlers
type Handler struct{}

// New creates a new calendar Handler
func New() *Handler { return &Handler{} }

// Databases returns available calendar databases (countries)
// GET /api/dashboard/calendar/databases
func (h *Handler) Databases(c *fiber.Ctx) error {
	return utils.Success(c, "success", []fiber.Map{
		{"id": 1, "code": "jo", "name": "الأردن"},
		{"id": 2, "code": "sa", "name": "السعودية"},
		{"id": 3, "code": "eg", "name": "مصر"},
		{"id": 4, "code": "ps", "name": "فلسطين"},
	})
}

// GetEvents returns calendar events
// GET /api/dashboard/calendar/events
func (h *Handler) GetEvents(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	countryCode, _ := c.Locals("country_code").(string)

	var events []models.Event
	query := db.Model(&models.Event{}).Where("database = ?", countryCode)

	if start := c.Query("start"); start != "" {
		query = query.Where("start_date >= ?", start)
	}
	if end := c.Query("end"); end != "" {
		query = query.Where("start_date <= ?", end)
	}

	query.Order("start_date ASC").Find(&events)
	return utils.Success(c, "success", events)
}

// CreateEvent creates a calendar event
// POST /api/dashboard/calendar/events
func (h *Handler) CreateEvent(c *fiber.Ctx) error {
	type CreateRequest struct {
		Title       string `json:"title" validate:"required,min=2,max=500"`
		Description string `json:"description"`
		StartDate   string `json:"start_date" validate:"required"`
		EndDate     string `json:"end_date"`
		AllDay      bool   `json:"all_day"`
		Color       string `json:"color"`
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
	countryCode, _ := c.Locals("country_code").(string)
	user, _ := c.Locals("user").(*models.User)

	event := models.Event{
		Title:    utils.SanitizeInput(req.Title),
		AllDay:   req.AllDay,
		Database: countryCode,
	}

	if req.Description != "" {
		event.Description = &req.Description
	}
	if req.Color != "" {
		event.Color = &req.Color
	}
	if user != nil {
		event.CreatedBy = &user.ID
	}

	if err := db.Create(&event).Error; err != nil {
		return utils.InternalError(c, "فشل إنشاء الحدث")
	}

	return utils.Created(c, "تم إنشاء الحدث بنجاح", event)
}

// UpdateEvent updates a calendar event
// PUT /api/dashboard/calendar/events/:id
func (h *Handler) UpdateEvent(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var event models.Event
	if err := db.First(&event, id).Error; err != nil {
		return utils.NotFound(c)
	}

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	db.Model(&event).Updates(updates)
	return utils.Success(c, "تم تحديث الحدث بنجاح", event)
}

// DeleteEvent deletes a calendar event
// DELETE /api/dashboard/calendar/events/:id
func (h *Handler) DeleteEvent(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	db.Delete(&models.Event{}, id)

	return utils.Success(c, "تم حذف الحدث بنجاح", nil)
}

// PublicEvents returns public calendar events
// GET /api/home/calendar
func (h *Handler) PublicEvents(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)
	countryCode, _ := c.Locals("country_code").(string)

	var events []models.Event
	db.Where("database = ?", countryCode).Order("start_date ASC").Limit(20).Find(&events)

	return utils.Success(c, "success", events)
}

// PublicEventDetail returns a single public event
// GET /api/home/event/:id
func (h *Handler) PublicEventDetail(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)
	db := database.DBForCountry(countryID)

	var event models.Event
	if err := db.First(&event, id).Error; err != nil {
		return utils.NotFound(c)
	}

	return utils.Success(c, "success", event)
}
