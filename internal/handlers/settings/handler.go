package settings

import (
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains settings route handlers
type Handler struct{}

// New creates a new settings Handler
func New() *Handler { return &Handler{} }

// GetAll returns all settings grouped by category
// GET /api/dashboard/settings
func (h *Handler) GetAll(c *fiber.Ctx) error {
	db := database.DB()
	var settings []models.Setting
	db.Order("group, key").Find(&settings)

	// Group settings by group key
	grouped := make(map[string][]models.Setting)
	for _, s := range settings {
		grouped[s.Group] = append(grouped[s.Group], s)
	}

	return utils.Success(c, "success", grouped)
}

// Update saves settings
// POST /api/dashboard/settings
func (h *Handler) Update(c *fiber.Ctx) error {
	var body map[string]string
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	db := database.DB()
	for key, value := range body {
		v := value
		db.Where(models.Setting{Key: key}).
			Assign(models.Setting{Value: &v}).
			FirstOrCreate(&models.Setting{})
	}

	user, _ := c.Locals("user").(*models.User)
	if user != nil {
		services.LogActivity("حدّث الإعدادات", "Setting", 0, user.ID)
	}

	return utils.Success(c, "تم حفظ الإعدادات بنجاح", nil)
}

// TestSMTP tests the SMTP connection
// POST /api/dashboard/settings/smtp/test
func (h *Handler) TestSMTP(c *fiber.Ctx) error {
	mailSvc := services.NewMailService()
	if err := mailSvc.TestSMTP(); err != nil {
		return utils.BadRequest(c, "فشل الاتصال بخادم البريد: "+err.Error())
	}
	return utils.Success(c, "تم الاتصال بخادم البريد بنجاح", nil)
}

// SendTestEmail sends a test email to the current user
// POST /api/dashboard/settings/smtp/send-test
func (h *Handler) SendTestEmail(c *fiber.Ctx) error {
	user, _ := c.Locals("user").(*models.User)
	if user == nil {
		return utils.Unauthorized(c)
	}

	mailSvc := services.NewMailService()
	if err := mailSvc.Send(user.Email, "رسالة اختبار - Alemancenter",
		"<p>هذه رسالة اختبار لإعدادات البريد الإلكتروني.</p>", true); err != nil {
		return utils.BadRequest(c, "فشل إرسال البريد: "+err.Error())
	}

	return utils.Success(c, "تم إرسال رسالة الاختبار بنجاح", nil)
}

// UpdateRobots updates the robots.txt content
// POST /api/dashboard/settings/robots
func (h *Handler) UpdateRobots(c *fiber.Ctx) error {
	type RobotsRequest struct {
		Content string `json:"content" validate:"required"`
	}

	var req RobotsRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	db := database.DB()
	content := req.Content
	db.Where(models.Setting{Key: "robots_txt"}).
		Assign(models.Setting{Value: &content}).
		FirstOrCreate(&models.Setting{})

	return utils.Success(c, "تم تحديث ملف robots.txt بنجاح", nil)
}

// GetPublic returns public-facing settings
// GET /api/front/settings
func (h *Handler) GetPublic(c *fiber.Ctx) error {
	db := database.DB()
	var settings []models.Setting
	db.Where("group = ?", "general").Find(&settings)

	result := make(map[string]string)
	for _, s := range settings {
		if s.Value != nil {
			result[s.Key] = *s.Value
		}
	}

	return utils.Success(c, "success", result)
}
