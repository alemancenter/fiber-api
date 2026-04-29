package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

const userCacheTTL = 1 * time.Minute

// loadUserCached returns the user from Redis cache or falls back to DB.
// On a cache miss it loads with Preload("Roles.Permissions","Permissions") and writes the result to cache.
func loadUserCached(userID uint) (*models.User, error) {
	ctx := context.Background()
	rdb := database.Redis()
	key := rdb.Key("user", fmt.Sprintf("%d", userID))

	if cached, err := rdb.Get(ctx, key); err == nil {
		var user models.User
		if json.Unmarshal([]byte(cached), &user) == nil {
			return &user, nil
		}
	}

	db := database.DB()
	var user models.User
	if err := db.Preload("Roles.Permissions").Preload("Permissions").
		First(&user, userID).Error; err != nil {
		return nil, err
	}

	if data, err := json.Marshal(user); err == nil {
		_ = rdb.Set(ctx, key, data, userCacheTTL)
	}
	return &user, nil
}

// InvalidateUserCache delegates to services.InvalidateUserCache.
// Kept as a shim so existing callers in this package don't need to import services.
func InvalidateUserCache(userID uint) { services.InvalidateUserCache(userID) }

// Auth validates JWT bearer token and loads the current user (with Redis caching).
func Auth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return utils.Unauthorized(c)
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		jwtSvc := services.NewJWTService()

		claims, err := jwtSvc.ValidateToken(tokenStr)
		if err != nil {
			return utils.Unauthorized(c, "رمز المصادقة غير صالح أو منتهي الصلاحية")
		}

		// Check token blacklist (tokens invalidated by logout)
		ctx := context.Background()
		rdb := database.Redis()
		hash := sha256.Sum256([]byte(tokenStr))
		blacklistKey := rdb.Key("blacklist", fmt.Sprintf("%x", hash))
		if exists, _ := rdb.Exists(ctx, blacklistKey); exists {
			return utils.Unauthorized(c, "رمز المصادقة غير صالح أو منتهي الصلاحية")
		}

		// Load user from Redis cache (falls back to DB on miss)
		user, err := loadUserCached(claims.UserID)
		if err != nil {
			return utils.Unauthorized(c, "المستخدم غير موجود")
		}

		if !user.IsActive() {
			return utils.Unauthorized(c, "الحساب غير نشط أو محظور")
		}

		c.Locals("user", user)
		c.Locals("user_id", user.ID)
		// If country_id was not set by FrontendGuard (e.g. mobile client) but the
		// token carries one, use it so downstream handlers always have a country.
		if c.Locals("country_id") == nil && claims.CountryID != 0 {
			c.Locals("country_id", database.CountryID(claims.CountryID))
		}
		return c.Next()
	}
}

// OptionalAuth loads user if token present, but doesn't require it (with Redis caching).
func OptionalAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Next()
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		jwtSvc := services.NewJWTService()
		claims, err := jwtSvc.ValidateToken(tokenStr)
		if err != nil {
			return c.Next()
		}

		if user, err := loadUserCached(claims.UserID); err == nil {
			c.Locals("user", user)
			c.Locals("user_id", user.ID)
		}
		return c.Next()
	}
}

// Can is a permission-based authorization middleware
func Can(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := c.Locals("user").(*models.User)
		if !ok || user == nil {
			return utils.Unauthorized(c)
		}

		if !user.HasPermission(permission) && !user.IsAdmin() {
			return utils.Forbidden(c)
		}

		return c.Next()
	}
}

// HasRole checks if the authenticated user has a specific role
func HasRole(role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := c.Locals("user").(*models.User)
		if !ok || user == nil {
			return utils.Unauthorized(c)
		}

		if !user.HasRole(role) && !user.IsAdmin() {
			return utils.Forbidden(c)
		}

		return c.Next()
	}
}

// AdminOnly ensures only admins can access the route
func AdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := c.Locals("user").(*models.User)
		if !ok || user == nil {
			return utils.Unauthorized(c)
		}

		if !user.IsAdmin() {
			return utils.Forbidden(c)
		}

		return c.Next()
	}
}

// GetUser retrieves the authenticated user from context
func GetUser(c *fiber.Ctx) *models.User {
	user, _ := c.Locals("user").(*models.User)
	return user
}
