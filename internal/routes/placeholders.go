package routes

import (
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// --- Inline mini-handlers for simple endpoints ---

func homeHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(utils.APIResponse{
			Success: true,
			Message: "مرحباً بك في Alemancenter API",
		})
	}
}

func legalHandler(page string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(utils.APIResponse{
			Success: true,
			Message: page,
		})
	}
}

func langChangeHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		type LangRequest struct {
			Locale string `json:"locale"`
		}
		var req LangRequest
		c.BodyParser(&req)
		return c.JSON(utils.APIResponse{Success: true, Message: req.Locale})
	}
}

func langCurrentHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		lang := c.Get("Accept-Language", "ar")
		if len(lang) >= 2 {
			lang = lang[:2]
		}
		return c.JSON(utils.APIResponse{Success: true, Message: lang})
	}
}
