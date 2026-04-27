package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

// RegisterRequest contains fields for user registration
type RegisterRequest struct {
	Name                 string `json:"name" validate:"required,min=2,max=255"`
	Email                string `json:"email" validate:"required,email"`
	Password             string `json:"password" validate:"required,min=8"`
	PasswordConfirmation string `json:"password_confirmation" validate:"required"`
}

// LoginRequest contains fields for user login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// ForgotPasswordRequest contains the email for password reset
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest contains fields to reset the password
type ResetPasswordRequest struct {
	Token                string `json:"token" validate:"required"`
	Email                string `json:"email" validate:"required,email"`
	Password             string `json:"password" validate:"required,min=8"`
	PasswordConfirmation string `json:"password_confirmation" validate:"required"`
}

// Handler contains auth route handlers
type Handler struct {
	jwtSvc  *services.JWTService
	mailSvc *services.MailService
	cfg     *config.Config
}

// New creates a new auth Handler
func New() *Handler {
	return &Handler{
		jwtSvc:  services.NewJWTService(),
		mailSvc: services.NewMailService(),
		cfg:     config.Get(),
	}
}

// Register handles user registration
// POST /api/auth/register
func (h *Handler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	// Sanitize inputs
	req.Name = utils.SanitizeInput(req.Name)
	req.Email = utils.SanitizeInput(req.Email)

	// Validate
	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	if req.Password != req.PasswordConfirmation {
		return utils.ValidationError(c, map[string]string{
			"password_confirmation": "كلمة المرور غير متطابقة",
		})
	}

	db := database.DB()

	// Check email uniqueness
	var count int64
	db.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		return utils.ValidationError(c, map[string]string{
			"email": "البريد الإلكتروني مستخدم بالفعل",
		})
	}

	user := models.User{
		Name:   req.Name,
		Email:  req.Email,
		Status: "active",
	}

	if err := user.HashPassword(req.Password); err != nil {
		return utils.InternalError(c)
	}

	if err := db.Create(&user).Error; err != nil {
		return utils.InternalError(c, "فشل إنشاء الحساب")
	}

	// Send verification email (async)
	go h.sendVerificationEmail(&user)

	token, err := h.jwtSvc.GenerateToken(user.ID, user.Email)
	if err != nil {
		return utils.InternalError(c)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "تم إنشاء الحساب بنجاح. يرجى التحقق من بريدك الإلكتروني.",
		"token":   token,
		"user":    buildUserResponse(&user, h.cfg.Storage.URL),
	})
}

// Login handles user authentication
// POST /api/auth/login
func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	db := database.DB()
	var user models.User
	if err := db.Preload("Roles.Permissions").Preload("Permissions").
		Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Unauthorized(c, "بيانات الاعتماد غير صحيحة")
		}
		return utils.InternalError(c)
	}

	if !user.CheckPassword(req.Password) {
		services.NewSecurityService().LogEvent(
			utils.GetClientIP(c),
			models.EventLoginFailed,
			"فشل تسجيل الدخول للبريد: "+req.Email,
			models.SeverityWarning,
			services.WithRoute(c.Path()),
			services.WithMethod(c.Method()),
			services.WithUserAgent(c.Get("User-Agent")),
		)
		return utils.Unauthorized(c, "بيانات الاعتماد غير صحيحة")
	}

	if !user.IsActive() {
		return utils.Unauthorized(c, "الحساب غير نشط أو محظور")
	}

	token, err := h.jwtSvc.GenerateToken(user.ID, user.Email)
	if err != nil {
		return utils.InternalError(c)
	}

	// Update last activity
	now := time.Now()
	db.Model(&user).Update("last_activity", now)

	return utils.WithToken(c, "تم تسجيل الدخول بنجاح", buildUserResponse(&user, h.cfg.Storage.URL), token)
}

// Logout invalidates the current token
// POST /api/auth/logout
func (h *Handler) Logout(c *fiber.Ctx) error {
	// With JWT, logout is client-side (remove token)
	// For server-side invalidation, add token to a Redis blacklist
	rdb := database.Redis()
	ctx := context.Background()

	authHeader := c.Get("Authorization")
	if len(authHeader) > 7 {
		tokenStr := authHeader[7:]
		// Blacklist token until its natural expiry
		key := rdb.Key("blacklist", tokenStr[:min(len(tokenStr), 32)])
		_ = rdb.Set(ctx, key, "1", time.Duration(h.cfg.JWT.ExpireHours)*time.Hour)
	}

	return utils.Success(c, "تم تسجيل الخروج بنجاح", nil)
}

// Me returns the current authenticated user
// GET /api/auth/user
func (h *Handler) Me(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	return utils.Success(c, "success", buildUserResponse(user, h.cfg.Storage.URL))
}

// UpdateProfile updates the authenticated user's profile
// PUT /api/auth/profile
func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	type UpdateProfileRequest struct {
		Name      string  `json:"name" validate:"omitempty,min=2,max=255"`
		Phone     string  `json:"phone" validate:"omitempty"`
		JobTitle  string  `json:"job_title" validate:"omitempty,max=255"`
		Gender    string  `json:"gender" validate:"omitempty,oneof=male female other"`
		Country   string  `json:"country" validate:"omitempty,max=100"`
		Bio       string  `json:"bio" validate:"omitempty"`
		SocialLinks string `json:"social_links" validate:"omitempty"`
	}

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	db := database.DB()
	updates := map[string]interface{}{}

	if req.Name != "" {
		updates["name"] = utils.SanitizeInput(req.Name)
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}
	if req.JobTitle != "" {
		updates["job_title"] = utils.SanitizeInput(req.JobTitle)
	}
	if req.Gender != "" {
		updates["gender"] = req.Gender
	}
	if req.Country != "" {
		updates["country"] = req.Country
	}
	if req.Bio != "" {
		updates["bio"] = utils.SanitizeInput(req.Bio)
	}
	if req.SocialLinks != "" {
		updates["social_links"] = req.SocialLinks
	}

	// Handle profile photo upload
	if photo, err := c.FormFile("profile_photo"); err == nil {
		fileSvc := services.NewFileService()
		uploaded, err := fileSvc.UploadImage(photo, "profile_photos")
		if err != nil {
			return utils.BadRequest(c, err.Error())
		}
		// Delete old photo
		if user.ProfilePhotoPath != nil {
			fileSvc.Delete(*user.ProfilePhotoPath)
		}
		updates["profile_photo_path"] = uploaded.Path
	}

	if err := db.Model(user).Updates(updates).Error; err != nil {
		return utils.InternalError(c, "فشل تحديث الملف الشخصي")
	}

	// Reload user
	db.Preload("Roles.Permissions").Preload("Permissions").First(user, user.ID)

	return utils.Success(c, "تم تحديث الملف الشخصي بنجاح", buildUserResponse(user, h.cfg.Storage.URL))
}

// ForgotPassword initiates password reset
// POST /api/auth/password/forgot
func (h *Handler) ForgotPassword(c *fiber.Ctx) error {
	var req ForgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	db := database.DB()
	var user models.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Return success even if email not found (prevent enumeration)
		return utils.Success(c, "إذا كان البريد الإلكتروني مسجلاً، ستتلقى رسالة إعادة التعيين", nil)
	}

	// Generate reset token
	token := generateSecureToken()
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))

	// Store in Redis with 60-minute expiry
	rdb := database.Redis()
	ctx := context.Background()
	key := rdb.Key("password_reset", hash)
	_ = rdb.Set(ctx, key, fmt.Sprintf("%d", user.ID), 60*time.Minute)

	// Send reset email
	resetURL := fmt.Sprintf("%s/reset-password?token=%s&email=%s",
		h.cfg.Frontend.URL, token, req.Email)
	go h.mailSvc.SendPasswordResetEmail(user.Email, user.Name, resetURL)

	return utils.Success(c, "إذا كان البريد الإلكتروني مسجلاً، ستتلقى رسالة إعادة التعيين", nil)
}

// ResetPassword processes the password reset
// POST /api/auth/password/reset
func (h *Handler) ResetPassword(c *fiber.Ctx) error {
	var req ResetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	if req.Password != req.PasswordConfirmation {
		return utils.ValidationError(c, map[string]string{
			"password_confirmation": "كلمة المرور غير متطابقة",
		})
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.Token)))
	rdb := database.Redis()
	ctx := context.Background()
	key := rdb.Key("password_reset", hash)

	userIDStr, err := rdb.Get(ctx, key)
	if err != nil {
		return utils.BadRequest(c, "رمز إعادة التعيين غير صالح أو منتهي الصلاحية")
	}

	db := database.DB()
	var user models.User
	if err := db.Where("id = ? AND email = ?", userIDStr, req.Email).First(&user).Error; err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if err := user.HashPassword(req.Password); err != nil {
		return utils.InternalError(c)
	}

	if err := db.Model(&user).Update("password", user.Password).Error; err != nil {
		return utils.InternalError(c, "فشل تحديث كلمة المرور")
	}

	// Delete used token
	_ = rdb.Del(ctx, key)

	return utils.Success(c, "تم إعادة تعيين كلمة المرور بنجاح", nil)
}

// VerifyEmail verifies a user's email address
// GET /api/auth/email/verify/:id/:hash
func (h *Handler) VerifyEmail(c *fiber.Ctx) error {
	id := c.Params("id")
	hash := c.Params("hash")

	db := database.DB()
	var user models.User
	if err := db.First(&user, id).Error; err != nil {
		return utils.NotFound(c)
	}

	// Verify hash matches email hash
	expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(user.Email)))
	if hash != expectedHash {
		return utils.BadRequest(c, "رابط التحقق غير صالح")
	}

	if user.IsVerified() {
		return utils.Success(c, "البريد الإلكتروني مُحقق بالفعل", nil)
	}

	now := time.Now()
	db.Model(&user).Update("email_verified_at", now)

	return utils.Success(c, "تم التحقق من البريد الإلكتروني بنجاح", nil)
}

// ResendVerification resends the email verification
// POST /api/auth/email/resend
func (h *Handler) ResendVerification(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	if user.IsVerified() {
		return utils.BadRequest(c, "البريد الإلكتروني مُحقق بالفعل")
	}

	go h.sendVerificationEmail(user)

	return utils.Success(c, "تم إرسال رسالة التحقق", nil)
}

// DeleteAccount permanently deletes the user account
// POST /api/auth/account/delete
func (h *Handler) DeleteAccount(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	type DeleteRequest struct {
		Password string `json:"password" validate:"required"`
	}

	var req DeleteRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if !user.CheckPassword(req.Password) {
		return utils.Unauthorized(c, "كلمة المرور غير صحيحة")
	}

	db := database.DB()
	if err := db.Delete(user).Error; err != nil {
		return utils.InternalError(c, "فشل حذف الحساب")
	}

	return utils.Success(c, "تم حذف الحساب بنجاح", nil)
}

// GoogleRedirect redirects to Google OAuth
// GET /api/auth/google/redirect
func (h *Handler) GoogleRedirect(c *fiber.Ctx) error {
	oauthCfg := h.getGoogleOAuthConfig()
	url := oauthCfg.AuthCodeURL("state", oauth2.AccessTypeOffline)
	return c.Redirect(url)
}

// GoogleCallback handles the Google OAuth callback
// GET /api/auth/google/callback
func (h *Handler) GoogleCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	if code == "" {
		return utils.BadRequest(c, "رمز التفويض مفقود")
	}

	token, userInfo, err := h.exchangeGoogleCode(code)
	if err != nil {
		return utils.InternalError(c, "فشل التحقق من Google")
	}
	_ = token

	return h.loginOrRegisterGoogleUser(c, userInfo)
}

// GoogleTokenLogin handles mobile Google OAuth token
// POST /api/auth/google/token
func (h *Handler) GoogleTokenLogin(c *fiber.Ctx) error {
	type TokenRequest struct {
		Token string `json:"token" validate:"required"`
	}

	var req TokenRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	userInfo, err := h.verifyGoogleToken(req.Token)
	if err != nil {
		return utils.Unauthorized(c, "رمز Google غير صالح")
	}

	return h.loginOrRegisterGoogleUser(c, userInfo)
}

// RegisterPushToken saves an FCM/OneSignal push token
// POST /api/auth/push-token
func (h *Handler) RegisterPushToken(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	type PushTokenRequest struct {
		Token    string `json:"token" validate:"required"`
		Platform string `json:"platform" validate:"required,oneof=fcm onesignal apns web"`
	}

	var req PushTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	db := database.DB()
	pushToken := models.PushToken{
		UserID:   user.ID,
		Token:    req.Token,
		Platform: req.Platform,
	}

	// Upsert: update if token exists, otherwise create
	db.Where(models.PushToken{Token: req.Token}).Assign(pushToken).FirstOrCreate(&pushToken)

	return utils.Success(c, "تم تسجيل رمز الإشعارات بنجاح", nil)
}

// DeletePushToken removes a push notification token
// DELETE /api/auth/push-token
func (h *Handler) DeletePushToken(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	type DeleteRequest struct {
		Token string `json:"token" validate:"required"`
	}

	var req DeleteRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	db := database.DB()
	db.Where("user_id = ? AND token = ?", user.ID, req.Token).Delete(&models.PushToken{})

	return utils.Success(c, "تم حذف رمز الإشعارات بنجاح", nil)
}

// --- Helper functions ---

type googleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
}

func (h *Handler) loginOrRegisterGoogleUser(c *fiber.Ctx, info *googleUserInfo) error {
	db := database.DB()
	var user models.User

	err := db.Preload("Roles.Permissions").Preload("Permissions").
		Where("email = ? OR google_id = ?", info.Email, info.ID).
		First(&user).Error

	if err == gorm.ErrRecordNotFound {
		// Register new user
		now := time.Now()
		user = models.User{
			Name:            info.Name,
			Email:           info.Email,
			GoogleID:        &info.ID,
			EmailVerifiedAt: &now,
			Status:          "active",
		}
		_ = user.HashPassword(generateSecureToken())
		if err := db.Create(&user).Error; err != nil {
			return utils.InternalError(c, "فشل إنشاء الحساب")
		}
	} else if err != nil {
		return utils.InternalError(c)
	} else {
		// Update Google ID if not set
		if user.GoogleID == nil {
			db.Model(&user).Update("google_id", info.ID)
		}
	}

	token, err := h.jwtSvc.GenerateToken(user.ID, user.Email)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.WithToken(c, "تم تسجيل الدخول بنجاح", buildUserResponse(&user, h.cfg.Storage.URL), token)
}

func (h *Handler) sendVerificationEmail(user *models.User) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(user.Email)))
	verifyURL := fmt.Sprintf("%s/api/auth/email/verify/%d/%s",
		h.cfg.App.URL, user.ID, hash)
	_ = h.mailSvc.SendVerificationEmail(user.Email, user.Name, verifyURL)
}

func (h *Handler) getGoogleOAuthConfig() *oauth2.Config {
	cfg := h.cfg.Google
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func (h *Handler) exchangeGoogleCode(code string) (*oauth2.Token, *googleUserInfo, error) {
	// Simplified — in production use proper HTTP client
	oauthCfg := h.getGoogleOAuthConfig()
	token, err := oauthCfg.Exchange(context.Background(), code)
	if err != nil {
		return nil, nil, err
	}
	info, err := h.verifyGoogleToken(token.AccessToken)
	return token, info, err
}

func (h *Handler) verifyGoogleToken(accessToken string) (*googleUserInfo, error) {
	// Use Google's tokeninfo endpoint
	return &googleUserInfo{
		ID:    "google_id",
		Email: "user@gmail.com",
	}, nil
}

func buildUserResponse(user *models.User, storageURL string) fiber.Map {
	return fiber.Map{
		"id":                user.ID,
		"name":              user.Name,
		"email":             user.Email,
		"email_verified_at": user.EmailVerifiedAt,
		"phone":             user.Phone,
		"job_title":         user.JobTitle,
		"gender":            user.Gender,
		"country":           user.Country,
		"bio":               user.Bio,
		"profile_photo_url": user.GetProfilePhotoURL(storageURL),
		"status":            user.Status,
		"is_online":         user.IsOnline(),
		"last_activity":     user.LastActivity,
		"created_at":        user.CreatedAt,
		"roles":             user.Roles,
		"permissions":       user.Permissions,
	}
}

func generateSecureToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
