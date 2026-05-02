package settings

import (
	"encoding/json"
	"fmt"
	"html"
	"net/mail"
	"regexp"
	"strings"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

var (
	adsenseClientRe  = regexp.MustCompile(`^ca-pub-\d+$`)
	adAllowedKeys    = map[string]bool{"ad_slot": true, "format": true, "responsive": true}
	adForbiddenWords = []string{"<script", "<iframe", "javascript:", "data:", "vbscript:"}
)

// validateAdUpdates rejects any google_ads_* value that is not empty or a safe
// JSON object containing only {ad_slot, format, responsive}.
func validateAdUpdates(updates map[string]string) error {
	for key, value := range updates {
		if key == "adsense_client" {
			if value != "" && !adsenseClientRe.MatchString(strings.TrimSpace(value)) {
				return fmt.Errorf("adsense_client: invalid format, expected ca-pub-XXXXXXXXXX")
			}
			continue
		}
		if !strings.HasPrefix(key, "google_ads_") {
			continue
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		lower := strings.ToLower(value)
		for _, forbidden := range adForbiddenWords {
			if strings.Contains(lower, forbidden) {
				return fmt.Errorf("%s: forbidden content", key)
			}
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			return fmt.Errorf("%s: must be empty or a JSON object {ad_slot, format, responsive}", key)
		}
		for k := range parsed {
			if !adAllowedKeys[k] {
				return fmt.Errorf("%s: unknown field '%s'", key, k)
			}
		}
		slot, ok := parsed["ad_slot"].(string)
		if !ok || strings.TrimSpace(slot) == "" {
			return fmt.Errorf("%s: ad_slot must be a non-empty string", key)
		}
	}
	return nil
}

// Handler contains settings route handlers
type Handler struct {
	svc services.SettingService
}

// New creates a new settings Handler
func New(svc services.SettingService) *Handler {
	return &Handler{svc: svc}
}

func countryIDFromContext(c *fiber.Ctx) database.CountryID {
	if countryID, ok := c.Locals("country_id").(database.CountryID); ok && countryID != 0 {
		return countryID
	}
	return database.CountryJordan
}

// GetAll returns all settings as a flat key→value map for the dashboard.
// @Summary Get Dashboard Settings
// @Description Returns all system settings as a key-value map for the admin dashboard
// @Tags Settings
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Param X-Country-Id header string false "Country ID"
// @Success 200 {object} utils.APIResponse{data=map[string]string}
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/settings [get]
func (h *Handler) GetAll(c *fiber.Ctx) error {
	m, err := h.svc.GetAll(c.Context(), countryIDFromContext(c))
	if err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "success", m)
}

// allowedSettingImageKeys lists the setting keys that accept file uploads.
var allowedSettingImageKeys = map[string]bool{
	"site_logo":     true,
	"site_favicon":  true,
	"site_og_image": true,
}

// Update saves settings using a batch upsert (INSERT … ON DUPLICATE KEY UPDATE).
// Accepts both application/json (key/value map) and multipart/form-data (for image uploads).
// @Summary Update Settings
// @Description Update system settings. Supports both JSON and Multipart Form Data for logo/favicon uploads.
// @Tags Settings
// @Accept json
// @Accept mpfd
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Param X-Country-Id header string false "Country ID"
// @Param settings body map[string]string false "Settings key-value pairs (if JSON)"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Router /dashboard/settings [post]
func (h *Handler) Update(c *fiber.Ctx) error {
	updates := make(map[string]string)

	ct := string(c.Request().Header.ContentType())
	if strings.Contains(ct, "multipart/form-data") {
		// Parse multipart: handle text fields and image file fields
		form, err := c.MultipartForm()
		if err != nil {
			return utils.BadRequest(c, "بيانات غير صحيحة")
		}
		for key, vals := range form.Value {
			if len(vals) > 0 {
				updates[key] = vals[0]
			}
		}
		fileRepo := repositories.NewFileRepository()
		fileSvc := services.NewFileService(fileRepo)
		for key, files := range form.File {
			if !allowedSettingImageKeys[key] || len(files) == 0 {
				continue
			}
			if uploaded, err := fileSvc.UploadImage(files[0], "settings"); err == nil {
				updates[key] = uploaded.Path
			}
		}
	} else {
		var body map[string]string
		if err := c.BodyParser(&body); err != nil {
			return utils.BadRequest(c, "بيانات غير صحيحة")
		}
		updates = body
	}

	if len(updates) == 0 {
		return utils.BadRequest(c, "لا توجد بيانات للحفظ")
	}

	if err := validateAdUpdates(updates); err != nil {
		return utils.BadRequest(c, err.Error())
	}

	var userID uint
	if user, ok := c.Locals("user").(*models.User); ok && user != nil {
		userID = user.ID
	}

	if err := h.svc.Update(c.Context(), countryIDFromContext(c), updates, userID); err != nil {
		return utils.InternalError(c, "فشل حفظ الإعدادات")
	}

	return utils.Success(c, "تم حفظ الإعدادات بنجاح", updates)
}

// TestSMTP tests the SMTP connection
// @Summary Test SMTP Connection
// @Description Tests the configured SMTP server connection
// @Tags Settings
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Router /dashboard/settings/smtp/test [post]
func (h *Handler) TestSMTP(c *fiber.Ctx) error {
	mailSvc := services.NewMailService()
	if err := mailSvc.TestSMTP(); err != nil {
		return utils.BadRequest(c, "فشل الاتصال بخادم البريد: "+err.Error())
	}
	return utils.Success(c, "تم الاتصال بخادم البريد بنجاح", nil)
}

// SendTestEmail sends a test email to the current user
// @Summary Send Test Email
// @Description Sends a test email via SMTP to the authenticated user's email address
// @Tags Settings
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Success 200 {object} utils.APIResponse
// @Failure 401 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Router /dashboard/settings/smtp/send-test [post]
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

// RobotsRequest represents the robots.txt update payload
type RobotsRequest struct {
	Content string `json:"content" validate:"required"`
}

// UpdateRobots updates the robots.txt content
// @Summary Update robots.txt
// @Description Updates the site's robots.txt content
// @Tags Settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security FrontendKeyAuth
// @Param X-Country-Id header string false "Country ID"
// @Param body body RobotsRequest true "robots.txt content"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Router /dashboard/settings/robots [post]
func (h *Handler) UpdateRobots(c *fiber.Ctx) error {
	var req RobotsRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	var userID uint
	if user, ok := c.Locals("user").(*models.User); ok && user != nil {
		userID = user.ID
	}

	if err := h.svc.Update(c.Context(), countryIDFromContext(c), map[string]string{"robots_txt": req.Content}, userID); err != nil {
		return utils.InternalError(c, "فشل تحديث ملف robots.txt")
	}

	return utils.Success(c, "تم تحديث ملف robots.txt بنجاح", nil)
}

// GetPublic returns public-facing settings.
// Result is cached in Redis for settingsCacheTTL to avoid a full table scan on every request.
// @Summary Get Public Settings
// @Description Returns public-facing settings required by the frontend application
// @Tags Public
// @Produce json
// @Param X-Country-Id header string false "Country ID"
// @Success 200 {object} utils.APIResponse{data=map[string]string}
// @Router /front/settings [get]
func (h *Handler) GetPublic(c *fiber.Ctx) error {
	result, err := h.svc.GetPublic(c.Context(), countryIDFromContext(c))
	if err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "success", result)
}

// ContactRequest represents the contact form data
type ContactRequest struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Subject   string `json:"subject"`
	Message   string `json:"message"`
	Recaptcha string `json:"g-recaptcha-response"`
	PageURL   string `json:"page_url"`
	FormTime  int64  `json:"form_time_ms"`
}

// Contact accepts public contact form submissions.
// @Summary Submit Contact Form
// @Description Accepts public contact form submissions and sends an email to the site administrator
// @Tags Public
// @Accept json
// @Produce json
// @Param X-Country-Id header string false "Country ID"
// @Param request body ContactRequest true "Contact form data"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /front/contact [post]
func (h *Handler) Contact(c *fiber.Ctx) error {
	var req ContactRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid contact payload")
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Subject = strings.TrimSpace(req.Subject)
	req.Message = strings.TrimSpace(req.Message)
	req.Recaptcha = strings.TrimSpace(req.Recaptcha)
	req.PageURL = strings.TrimSpace(req.PageURL)

	if req.Name == "" || req.Email == "" || req.Subject == "" || req.Message == "" {
		return utils.BadRequest(c, "name, email, subject and message are required")
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return utils.BadRequest(c, "invalid email address")
	}
	if req.Recaptcha == "" {
		return utils.BadRequest(c, "recaptcha token is required")
	}
	if req.FormTime > 0 && req.FormTime < 1200 {
		return utils.BadRequest(c, "contact form submitted too quickly")
	}

	settings, _ := h.svc.GetPublic(c.Context(), countryIDFromContext(c))
	recipient := firstSetting(settings, "contact_email", "site_email")
	if recipient == "" {
		recipient = config.Get().Mail.FromAddress
	}
	if recipient == "" {
		return utils.BadRequest(c, "contact email is not configured")
	}

	body := fmt.Sprintf(`
<div dir="rtl" style="font-family: Arial, sans-serif; line-height: 1.8">
  <h2>Contact form message</h2>
  <p><strong>Name:</strong> %s</p>
  <p><strong>Email:</strong> %s</p>
  <p><strong>Phone:</strong> %s</p>
  <p><strong>Page:</strong> %s</p>
  <hr>
  <p>%s</p>
</div>`,
		html.EscapeString(req.Name),
		html.EscapeString(req.Email),
		html.EscapeString(req.Phone),
		html.EscapeString(req.PageURL),
		html.EscapeString(req.Message),
	)

	subject := "Contact form: " + req.Subject
	if err := services.NewMailService().Send(recipient, subject, body, true); err != nil {
		return utils.InternalError(c, "failed to send contact message")
	}

	return utils.Success(c, "contact message sent successfully", nil)
}

func firstSetting(settings map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(settings[key]); value != "" {
			return value
		}
	}
	return ""
}
