package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORS configures Cross-Origin Resource Sharing
func CORS() fiber.Handler {
	return cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			// Allow all for development testing
			return true
		},
		AllowOrigins: "",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders: strings.Join([]string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Requested-With",
			"X-Frontend-Key",
			"X-Country-Id",
			"X-Country-Code",
			"X-App-Locale",
			"X-CSRF-Token",
			"Cache-Control",
		}, ","),
		ExposeHeaders: strings.Join([]string{
			"X-Total-Count",
			"X-Page",
			"X-Per-Page",
			"Content-Disposition",
		}, ","),
		AllowCredentials: true,
		MaxAge:           86400,
	})
}
