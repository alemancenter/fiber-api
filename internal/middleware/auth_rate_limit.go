package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/alemancenter/fiber-api/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// authLimitRule defines per-endpoint brute-force limits.
type authLimitRule struct {
	max    int
	window time.Duration
}

// authLimits maps endpoint suffix → rule. Keys are matched as path suffixes.
var authLimits = map[string]authLimitRule{
	"/auth/login":           {max: 5, window: 15 * time.Minute},
	"/auth/register":        {max: 10, window: 15 * time.Minute},
	"/auth/password/forgot": {max: 3, window: 15 * time.Minute},
	"/auth/refresh":         {max: 10, window: time.Minute},
}

// AuthRateLimit applies a strict per-IP rate limit for sensitive auth endpoints.
// It is intentionally separate from the general FrontendGuard rate limiter so
// that auth routes can be tuned independently without affecting other API paths.
func AuthRateLimit() fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()

		rule, ok := authLimits[path]
		if !ok {
			return c.Next()
		}

		clientIP := utils.GetClientIP(c)
		rdb := database.GetRedis()
		ctx := context.Background()

		key := rdb.Key("auth_rl", clientIP, path)

		count, err := rdb.IncrBy(ctx, key, 1)
		if err != nil {
			logger.Error("auth rate limit Redis error — failing closed",
				zap.String("ip", clientIP),
				zap.String("path", path),
				zap.Error(err),
			)
			return utils.TooManyRequests(c)
		}

		if count == 1 {
			_ = rdb.Expire(ctx, key, rule.window)
		}

		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", rule.max))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, rule.max-int(count))))

		if int(count) > rule.max {
			retryAfter := int(rule.window.Seconds())
			c.Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			return utils.TooManyRequests(c)
		}

		return c.Next()
	}
}
