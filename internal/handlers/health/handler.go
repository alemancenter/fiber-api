package health

import (
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/gofiber/fiber/v2"
)

// Handler contains health check route handlers
type Handler struct {
	svc services.HealthService
}

// New creates a new health Handler
func New(svc services.HealthService) *Handler {
	return &Handler{
		svc: svc,
	}
}

// Ping returns a simple health check response
// GET /api/ping
func (h *Handler) Ping(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"message": "pong",
	})
}

// Health returns detailed health check information
// GET /api/health
func (h *Handler) Health(c *fiber.Ctx) error {
	data, httpStatus, _ := h.svc.GetHealthStatus()

	return c.Status(httpStatus).JSON(data)
}