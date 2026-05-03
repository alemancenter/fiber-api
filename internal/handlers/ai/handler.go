package ai

import (
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Handler contains AI route handlers.
type Handler struct {
	svc services.AIService
}

// New creates a new AI Handler.
func New(svc services.AIService) *Handler {
	return &Handler{svc: svc}
}

type GenerateRequest struct {
	Title string `json:"title" validate:"required,min=3,max=255"`
}

// Generate starts an async AI content generation job and returns a job ID immediately.
// The client should poll /ai/status/:id until status is "done" or "failed".
//
// @Summary Start AI Content Generation
// @Description Start async AI article content generation. Returns a job_id to poll for results.
// @Tags AI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Param request body GenerateRequest true "Article title"
// @Success 202 {object} map[string]interface{}
// @Failure 400 {object} utils.APIResponse
// @Router /ai/generate [post]
func (h *Handler) Generate(c *fiber.Ctx) error {
	var req GenerateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صالحة")
	}
	if len(req.Title) < 3 || len(req.Title) > 255 {
		return utils.BadRequest(c, "عنوان المقال يجب أن يكون بين 3 و 255 حرف")
	}

	jobID := uuid.New().String()
	store := services.GetAIJobStore()
	store.Create(jobID)

	// Run generation in the background — the HTTP response returns immediately.
	go func() {
		content, err := h.svc.GenerateArticleContent(req.Title)
		if err != nil {
			store.Fail(jobID, err.Error())
			return
		}
		store.Complete(jobID, content)
	}()

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"success": true,
		"job_id":  jobID,
		"status":  services.JobPending,
	})
}

// Status returns the current state of an async AI generation job.
//
// @Summary Poll AI Job Status
// @Description Poll for the result of a previously started AI generation job.
// @Tags AI
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Param id path string true "Job ID returned by /ai/generate"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.APIResponse
// @Failure 404 {object} utils.APIResponse
// @Router /ai/status/{id} [get]
func (h *Handler) Status(c *fiber.Ctx) error {
	jobID := c.Params("id")
	if jobID == "" {
		return utils.BadRequest(c, "معرف المهمة مطلوب")
	}

	job, ok := services.GetAIJobStore().Get(jobID)
	if !ok {
		return utils.NotFound(c, "المهمة غير موجودة أو انتهت صلاحيتها")
	}

	resp := fiber.Map{
		"success": true,
		"job_id":  job.ID,
		"status":  job.Status,
	}
	if job.Status == services.JobDone {
		resp["content"] = job.Content
	}
	if job.Status == services.JobFailed {
		resp["error"] = job.Error
	}

	return c.JSON(resp)
}
