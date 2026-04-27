package middleware

import (
	"strings"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Auth validates JWT bearer token and loads the current user
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

		// Load user from Jordan (main) database
		db := database.DB()
		var user models.User
		if err := db.
			Preload("Roles.Permissions").
			Preload("Permissions").
			First(&user, claims.UserID).Error; err != nil {
			return utils.Unauthorized(c, "المستخدم غير موجود")
		}

		if !user.IsActive() {
			return utils.Unauthorized(c, "الحساب غير نشط أو محظور")
		}

		// Store user in context
		c.Locals("user", &user)
		c.Locals("user_id", user.ID)

		return c.Next()
	}
}

// OptionalAuth loads user if token present, but doesn't require it
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

		db := database.DB()
		var user models.User
		if err := db.Preload("Roles.Permissions").Preload("Permissions").First(&user, claims.UserID).Error; err == nil {
			c.Locals("user", &user)
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
