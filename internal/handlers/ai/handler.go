package ai

import (
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc services.AIService
}

func New(svc services.AIService) *Handler {
	return &Handler{svc: svc}
}

// Generate creates content from a title
// POST /api/ai/generate
func (h *Handler) Generate(c *fiber.Ctx) error {
	type Request struct {
		Title string `json:"title" validate:"required,min=3,max=255"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صالحة")
	}

	// Validate permissions using middleware if needed, but the route might already be protected
	// We just proceed assuming it's protected by middleware or we can check manually
	// Actually, the route is inside auth and dashboard, which are protected.

	if len(req.Title) < 3 || len(req.Title) > 255 {
		return utils.BadRequest(c, "عنوان المقال يجب أن يكون بين 3 و 255 حرف")
	}

	content, err := h.svc.GenerateArticleContent(req.Title)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to generate content. Please try again later.",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"content": content,
	})
}
