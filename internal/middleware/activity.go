package middleware

import (
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// UpdateLastActivity updates the authenticated user's last activity timestamp
func UpdateLastActivity() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return err
		}

		user, ok := c.Locals("user").(*models.User)
		if !ok || user == nil {
			return nil
		}

		now := time.Now()
		// Only update if last activity was more than 1 minute ago (reduces DB writes)
		if user.LastActivity != nil && time.Since(*user.LastActivity) < time.Minute {
			return nil
		}

		go func() {
			db := database.DB()
			db.Model(user).Update("last_activity", now)
		}()

		return nil
	}
}

// TrackVisitor records visitor data for analytics
func TrackVisitor() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return err
		}

		// Only track successful GET requests (sampling)
		if c.Method() != "GET" || c.Response().StatusCode() >= 400 {
			return nil
		}

		// Sample 1 in 3 requests to reduce writes
		clientIP := utils.GetClientIP(c)
		if len(clientIP) > 0 && clientIP[0]%3 != 0 {
			return nil
		}

		countryCode, _ := c.Locals("country_code").(string)
		if countryCode == "" {
			countryCode = "jo"
		}

		var userID *uint
		if user, ok := c.Locals("user").(*models.User); ok && user != nil {
			uid := user.ID
			userID = &uid
		}

		go func() {
			db := database.GetManager().GetByCode(countryCode)
			tracking := models.VisitorTracking{
				IPAddress: clientIP,
				Page:      c.Path(),
				Database:  countryCode,
				UserID:    userID,
				CreatedAt: time.Now(),
			}
			if ua := c.Get("User-Agent"); ua != "" {
				tracking.UserAgent = &ua
			}
			if ref := c.Get("Referer"); ref != "" {
				tracking.Referer = &ref
			}
			db.Create(&tracking)
		}()

		return nil
	}
}
