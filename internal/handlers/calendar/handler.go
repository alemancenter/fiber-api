package calendar

import (
	"strconv"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains calendar route handlers
type Handler struct {
	svc services.CalendarService
}

// New creates a new calendar Handler
func New(svc services.CalendarService) *Handler {
	return &Handler{
		svc: svc,
	}
}

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

// GetEvents returns calendar events for the dashboard
// GET /api/dashboard/calendar/events
func (h *Handler) GetEvents(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)
	start := c.Query("start")
	end := c.Query("end")

	events, err := h.svc.ListEvents(countryID, start, end)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", events)
}

// CreateEvent creates a calendar event
// POST /api/dashboard/calendar/events
func (h *Handler) CreateEvent(c *fiber.Ctx) error {
	type CreateRequest struct {
		Title       string `json:"title" validate:"required,min=2,max=500"`
		Description string `json:"description"`
		EventDate   string `json:"event_date" validate:"required"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	countryID, _ := c.Locals("country_id").(database.CountryID)

	eventDate, err := time.Parse("2006-01-02", req.EventDate)
	if err != nil {
		return utils.BadRequest(c, "صيغة التاريخ غير صحيحة")
	}

	event := models.Event{
		Title:     utils.SanitizeInput(req.Title),
		EventDate: eventDate,
	}

	if req.Description != "" {
		event.Description = &req.Description
	}

	if err := h.svc.CreateEvent(countryID, &event); err != nil {
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

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	event, err := h.svc.UpdateEvent(countryID, id, updates)
	if err != nil {
		return utils.NotFound(c)
	}

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
	if err := h.svc.DeleteEvent(countryID, id); err != nil {
		return utils.NotFound(c)
	}

	return utils.Success(c, "تم حذف الحدث بنجاح", nil)
}

// PublicEvents returns upcoming calendar events
// GET /api/home/calendar
func (h *Handler) PublicEvents(c *fiber.Ctx) error {
	countryID, _ := c.Locals("country_id").(database.CountryID)

	events, err := h.svc.ListPublicEvents(countryID, 20)
	if err != nil {
		return utils.InternalError(c)
	}

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

	event, err := h.svc.GetEvent(countryID, id)
	if err != nil {
		return utils.NotFound(c)
	}

	return utils.Success(c, "success", event)
}
