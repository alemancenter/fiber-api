package middleware

import (
	"sync"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// activityCache is an in-process write-dedup map: userID → last DB write time.
// Stored BEFORE the goroutine fires so concurrent requests on the same user see
// the updated timestamp immediately and skip the redundant UPDATE.
var activityCache sync.Map

const activityDebounce = time.Minute

// UpdateLastActivity updates the authenticated user's last activity timestamp.
// At most one DB write per user per activityDebounce window, regardless of
// how many concurrent requests arrive.
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

		// LoadOrStore with a *sync.Mutex per user to make the check-and-set atomic
		type entry struct {
			mu   sync.Mutex
			last time.Time
		}
		v, _ := activityCache.LoadOrStore(user.ID, &entry{})
		e := v.(*entry)

		e.mu.Lock()
		skip := now.Sub(e.last) < activityDebounce
		if !skip {
			e.last = now // mark before unlock so other goroutines see it
		}
		e.mu.Unlock()

		if skip {
			return nil
		}

		go database.DB().Exec(
			"UPDATE users SET last_activity = ?, updated_at = ? WHERE id = ?",
			now, now, user.ID,
		)

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
			path := c.Path()
			now := time.Now()
			tracking := models.VisitorTracking{
				IPAddress:    clientIP,
				UserAgent:    c.Get("User-Agent"),
				URL:          &path,
				UserID:       userID,
				LastActivity: now,
				CreatedAt:    now,
			}
			if ref := c.Get("Referer"); ref != "" {
				tracking.Referer = &ref
			}
			db.Create(&tracking)
		}()

		return nil
	}
}
