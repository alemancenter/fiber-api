package middleware

import (
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// IPGuard blocks requests from blocked IPs and skips checks for trusted IPs
func IPGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		clientIP := utils.GetClientIP(c)

		// Localhost always passes
		if utils.IsLocalhost(clientIP) {
			return c.Next()
		}

		db := database.DB()

		// Check if IP is blocked
		var blocked models.BlockedIP
		if err := db.Where("ip_address = ?", clientIP).First(&blocked).Error; err == nil {
			if !blocked.IsExpired() {
				return utils.Forbidden(c, "عنوان IP الخاص بك محظور")
			}
			// Auto-remove expired block
			db.Delete(&blocked)
		}

		// Mark trusted status in context
		var trusted models.TrustedIP
		if err := db.Where("ip_address = ?", clientIP).First(&trusted).Error; err == nil {
			c.Locals("trusted_ip", true)
		}

		return c.Next()
	}
}
