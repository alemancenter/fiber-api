package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// FrontendGuard validates API requests are from authorized frontends.
// Mirrors Laravel's FrontendApiGuard middleware.
func FrontendGuard() fiber.Handler {
	cfg := config.Get()

	// Paths excluded from frontend guard validation
	excludedPaths := []string{
		"/api/auth/google/callback",
		"/api/auth/email/verify/",
		"/api/ping",
		"/api/img/fit/",
		"/api/secure/view",
	}

	return func(c *fiber.Ctx) error {
		path := c.Path()

		// Skip excluded paths
		for _, excluded := range excludedPaths {
			if strings.HasPrefix(path, excluded) {
				return c.Next()
			}
		}

		clientIP := utils.GetClientIP(c)
		origin := c.Get("Origin")
		frontendKey := c.Get("X-Frontend-Key")
		userAgent := c.Get("User-Agent")
		requestedWith := c.Get("X-Requested-With")
		referer := c.Get("Referer")
		authHeader := c.Get("Authorization")

		// 1. Localhost / development bypass
		if utils.IsLocalhost(clientIP) && requestedWith == "XMLHttpRequest" {
			c.Locals("client_type", "localhost")
			return continueWithCountry(c, cfg)
		}

		// 2. SSR (Server-Side Rendering) detection — Node.js/Next.js
		if utils.IsSSRUserAgent(userAgent) {
			isSSRTrusted := false
			for _, ip := range cfg.Frontend.SSRTrustedIPs {
				if strings.TrimSpace(ip) == clientIP {
					isSSRTrusted = true
					break
				}
			}
			if isSSRTrusted {
				c.Locals("client_type", "ssr")
				c.Locals("rate_limit_max", cfg.Frontend.SSRRateLimitMax)
				return continueWithCountry(c, cfg)
			}
		}

		// 3. Frontend API key validation (strongest signal)
		if cfg.Frontend.APIKey != "" && frontendKey == cfg.Frontend.APIKey {
			c.Locals("client_type", "api_key")
			return continueWithCountry(c, cfg)
		}

		// 4. Authenticated bearer token
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			c.Locals("client_type", "bearer")
			return continueWithCountry(c, cfg)
		}

		// 5. Origin + Referer validation (browser CORS)
		if origin != "" {
			if isAllowedOrigin(origin, cfg.Frontend.CORSOrigins) {
				if requestedWith == "XMLHttpRequest" || referer != "" {
					c.Locals("client_type", "browser")
					return continueWithCountry(c, cfg)
				}
			}
		}

		return utils.Unauthorized(c, "غير مصرح بالوصول لهذه الواجهة")
	}
}

// continueWithCountry sets the country database connection and proceeds
func continueWithCountry(c *fiber.Ctx, cfg *config.Config) error {
	countryHeader := c.Get("X-Country-Id")
	if countryHeader == "" {
		countryHeader = c.Query("country", "1")
	}

	countryID := database.CountryIDFromHeader(countryHeader)
	c.Locals("country_id", countryID)
	c.Locals("country_code", database.CountryCode(countryID))

	// Apply frontend rate limiting
	if cfg.Frontend.RateLimit {
		if err := applyRateLimit(c, cfg, countryID); err != nil {
			return err
		}
	}

	return c.Next()
}

// applyRateLimit applies Redis-backed rate limiting
func applyRateLimit(c *fiber.Ctx, cfg *config.Config, countryID database.CountryID) error {
	clientIP := utils.GetClientIP(c)
	maxReqs := cfg.Frontend.RateLimitMax
	window := cfg.Frontend.RateLimitWindow

	// SSR gets higher limit
	if limit, ok := c.Locals("rate_limit_max").(int); ok {
		maxReqs = limit
	}

	// Login endpoints get stricter limits
	path := c.Path()
	if strings.Contains(path, "/auth/login") || strings.Contains(path, "/auth/register") {
		maxReqs = cfg.Frontend.LoginRateLimit
		window = 60
	}

	rdb := database.GetRedis()
	ctx := context.Background()
	key := rdb.Key("ratelimit", clientIP, path)

	count, err := rdb.IncrBy(ctx, key, 1)
	if err != nil {
		return c.Next() // Fail open on Redis error
	}

	if count == 1 {
		_ = rdb.Expire(ctx, key, time.Duration(window)*time.Second)
	}

	c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", maxReqs))
	c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, maxReqs-int(count))))

	if int(count) > maxReqs {
		return utils.TooManyRequests(c)
	}

	return nil
}

// isAllowedOrigin checks if origin is in the allowed list
func isAllowedOrigin(origin string, allowed []string) bool {
	for _, a := range allowed {
		if strings.TrimSpace(a) == origin {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
