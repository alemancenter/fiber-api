package routes

import (
	"github.com/alemancenter/fiber-api/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func RegisterAdminRoutes(api fiber.Router, h *Handlers) {
	// =====================
	// ADMIN ROUTES
	// =====================
	admin := api.Group("/admin", middleware.Auth(), middleware.AdminOnly())

	// Redis management (admin only)
	adminRedis := admin.Group("/redis")
	adminRedis.Get("/keys", h.Redis.ListKeys)
	adminRedis.Post("", h.Redis.SetKey)
	adminRedis.Delete("/expired/clean", h.Redis.CleanExpired)
	adminRedis.Get("/test", h.Redis.TestConnection)
	adminRedis.Get("/info", h.Redis.GetInfo)
	adminRedis.Delete("/:key", h.Redis.DeleteKey)
}
