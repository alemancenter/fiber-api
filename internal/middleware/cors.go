package middleware

import (
	"strings"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORS configures Cross-Origin Resource Sharing
func CORS() fiber.Handler {
	cfg := config.Get()
	origins := strings.Join(cfg.Frontend.CORSOrigins, ",")

	return cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			for _, allowed := range cfg.Frontend.CORSOrigins {
				allowed = strings.TrimSpace(allowed)
				// Never match wildcard when credentials are enabled — browsers reject it anyway
				// and it would allow any origin to make credentialed cross-origin requests.
				if allowed == "*" {
					continue
				}
				if allowed == origin {
					return true
				}
			}
			return false
		},
		AllowOrigins: origins,
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
