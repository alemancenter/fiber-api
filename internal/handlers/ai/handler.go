package ai

import (
	_ "github.com/alemancenter/fiber-api/internal/models"
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

type GenerateRequest struct {
	Title string `json:"title" validate:"required,min=3,max=255"`
}

// Generate creates content from a title
// @Summary AI Content Generation
// @Description Generate article content automatically using AI based on a provided title
// @Tags AI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Param request body GenerateRequest true "Prompt payload containing title"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /ai/generate [post]
func (h *Handler) Generate(c *fiber.Ctx) error {
	var req GenerateRequest
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
