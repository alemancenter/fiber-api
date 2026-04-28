package routes

import (
	"github.com/alemancenter/fiber-api/internal/middleware"
	"github.com/gofiber/fiber/v2"
	fiberCompress "github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/etag"
)

// Setup registers all API routes on the given Fiber app
func Setup(app *fiber.App) {
	// Initialize Dependencies
	deps := NewDependencies()

	// Global middleware
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.CORS())
	app.Use(middleware.RequestLogger())
	app.Use(fiberCompress.New())
	app.Use(etag.New())

	// Health check (no auth required)
	app.Get("/api/ping", deps.Health.Ping)
	app.Get("/api/health", deps.Health.Health)

	// API group with frontend guard and IP guard
	api := app.Group("/api",
		middleware.IPGuard(),
		middleware.FrontendGuard(),
	)

	// Register separate route domains
	RegisterPublicRoutes(api, deps)
	RegisterAuthRoutes(api, deps)
	RegisterDashboardRoutes(api, deps)

	// We map the redis admin group inside dash as before but use AdminOnly via RegisterAdminRoutes if we want,
	// but let's keep it under dash to match the old path: /api/dashboard/redis
	dash := api.Group("/dashboard",
		middleware.Auth(),
		middleware.UpdateLastActivity(),
		middleware.DashboardSecurityHeaders(),
	)
	RegisterAdminRoutes(dash, deps)
}
