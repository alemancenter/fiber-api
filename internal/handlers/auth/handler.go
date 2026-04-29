package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
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
	svc services.AuthService
	cfg *config.Config
}

// New creates a new auth Handler
func New(svc services.AuthService) *Handler {
	return &Handler{
		svc: svc,
		cfg: config.Get(),
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

	user, token, err := h.svc.Register(req.Name, req.Email, req.Password)
	if err != nil {
		if err == services.ErrEmailAlreadyExists {
			return utils.ValidationError(c, map[string]string{
				"email": "البريد الإلكتروني مستخدم بالفعل",
			})
		}
		return utils.InternalError(c, "فشل إنشاء الحساب")
	}

	return c.Status(fiber.StatusCreated).JSON(services.RegisterResponse{
		Success: true,
		Message: "تم إنشاء الحساب بنجاح. يرجى التحقق من بريدك الإلكتروني.",
		Token:   token,
		User:    buildUserResponse(user, h.cfg.Storage.URL),
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

	user, token, err := h.svc.Login(
		req.Email,
		req.Password,
		utils.GetClientIP(c),
		c.Get("User-Agent"),
		c.Method(),
		c.Path(),
	)

	if err != nil {
		if err == services.ErrInvalidCredentials || err == services.ErrAccountInactive {
			return utils.Unauthorized(c, err.Error())
		}
		return utils.InternalError(c)
	}

	refreshToken, _ := h.svc.GenerateRefreshTokenForUser(user.ID, user.Email)

	return c.JSON(fiber.Map{
		"success":       true,
		"message":       "تم تسجيل الدخول بنجاح",
		"token":         token,
		"refresh_token": refreshToken,
		"data":          buildUserResponse(user, h.cfg.Storage.URL),
	})
}

// RefreshToken issues a new access token from a valid refresh token
// POST /api/auth/refresh
func (h *Handler) RefreshToken(c *fiber.Ctx) error {
	type RefreshRequest struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}
	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	accessToken, newRefresh, err := h.svc.RefreshToken(req.RefreshToken)
	if err != nil {
		return utils.Unauthorized(c, "رمز التحديث غير صالح أو منتهي الصلاحية")
	}

	return utils.Success(c, "تم تجديد الرمز بنجاح", fiber.Map{
		"token":         accessToken,
		"refresh_token": newRefresh,
	})
}

// Logout invalidates the current token
// POST /api/auth/logout
func (h *Handler) Logout(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	var tokenStr string
	if len(authHeader) > 7 {
		tokenStr = authHeader[7:]
	}

	var user *models.User
	if u, ok := c.Locals("user").(*models.User); ok && u != nil {
		user = u
	}

	if err := h.svc.Logout(tokenStr, user); err != nil {
		return utils.InternalError(c, "فشل تسجيل الخروج")
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

	var req services.UpdateProfileInput
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	if req.Name != "" {
		req.Name = utils.SanitizeInput(req.Name)
	}
	if req.JobTitle != nil && *req.JobTitle != "" {
		sanitized := utils.SanitizeInput(*req.JobTitle)
		req.JobTitle = &sanitized
	}
	if req.Bio != nil && *req.Bio != "" {
		sanitized := utils.SanitizeInput(*req.Bio)
		req.Bio = &sanitized
	}

	// Handle profile photo upload
	if photo, err := c.FormFile("profile_photo"); err == nil {
		fileRepo := repositories.NewFileRepository()
		fileSvc := services.NewFileService(fileRepo)
		uploaded, err := fileSvc.UploadImage(photo, "profile_photos")
		if err != nil {
			return utils.BadRequest(c, err.Error())
		}
		// Delete old photo
		if user.ProfilePhotoPath != nil {
			fileSvc.Delete(*user.ProfilePhotoPath)
		}
		req.ProfilePhotoPath = &uploaded.Path
	}

	updatedUser, err := h.svc.UpdateProfile(user, &req)
	if err != nil {
		return utils.InternalError(c, "فشل تحديث الملف الشخصي")
	}

	return utils.Success(c, "تم تحديث الملف الشخصي بنجاح", buildUserResponse(updatedUser, h.cfg.Storage.URL))
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

	// We don't care about the error, we always return success to prevent enumeration
	_ = h.svc.ForgotPassword(req.Email)

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

	err := h.svc.ResetPassword(req.Token, req.Email, req.Password)
	if err != nil {
		if err == services.ErrInvalidResetToken || err == services.ErrInvalidCredentials {
			return utils.BadRequest(c, err.Error())
		}
		return utils.InternalError(c, "فشل تحديث كلمة المرور")
	}

	return utils.Success(c, "تم إعادة تعيين كلمة المرور بنجاح", nil)
}

// VerifyEmail verifies a user's email address
// GET /api/auth/email/verify/:id/:hash
func (h *Handler) VerifyEmail(c *fiber.Ctx) error {
	id := c.Params("id")
	hash := c.Params("hash")

	err := h.svc.VerifyEmail(id, hash)
	if err != nil {
		if err == services.ErrAlreadyVerified {
			return utils.Success(c, err.Error(), nil)
		}
		if err == services.ErrInvalidVerifyToken {
			return utils.BadRequest(c, err.Error())
		}
		return utils.NotFound(c)
	}

	return utils.Success(c, "تم التحقق من البريد الإلكتروني بنجاح", nil)
}

// ResendVerification resends the email verification
// POST /api/auth/email/resend
func (h *Handler) ResendVerification(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	err := h.svc.ResendVerification(user)
	if err != nil {
		if err == services.ErrAlreadyVerified {
			return utils.BadRequest(c, err.Error())
		}
		return utils.InternalError(c)
	}

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

	err := h.svc.DeleteAccount(user, req.Password)
	if err != nil {
		if err == services.ErrInvalidCredentials {
			return utils.Unauthorized(c, err.Error())
		}
		return utils.InternalError(c, "فشل حذف الحساب")
	}

	return utils.Success(c, "تم حذف الحساب بنجاح", nil)
}

// GoogleRedirect redirects to Google OAuth
// GET /api/auth/google/redirect
func (h *Handler) GoogleRedirect(c *fiber.Ctx) error {
	oauthCfg := h.svc.GetGoogleOAuthConfig()
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

	err := h.svc.UpsertPushToken(user.ID, req.Token, req.Platform)
	if err != nil {
		return utils.InternalError(c, "فشل تسجيل رمز الإشعارات")
	}

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

	err := h.svc.DeletePushToken(user.ID, req.Token)
	if err != nil {
		return utils.InternalError(c, "فشل حذف رمز الإشعارات")
	}

	return utils.Success(c, "تم حذف رمز الإشعارات بنجاح", nil)
}

// --- Helper functions ---

func (h *Handler) exchangeGoogleCode(code string) (*oauth2.Token, *services.GoogleUserInfo, error) {
	ctx := context.Background()
	oauthCfg := h.svc.GetGoogleOAuthConfig()

	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		return nil, nil, err
	}

	client := oauthCfg.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var userInfo services.GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, nil, err
	}

	return token, &userInfo, nil
}

func (h *Handler) verifyGoogleToken(token string) (*services.GoogleUserInfo, error) {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("invalid google token")
	}

	var userInfo services.GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

func (h *Handler) loginOrRegisterGoogleUser(c *fiber.Ctx, info *services.GoogleUserInfo) error {
	user, token, err := h.svc.LoginOrRegisterGoogleUser(info)
	if err != nil {
		return utils.InternalError(c, "فشل معالجة حساب Google")
	}

	return utils.WithToken(c, "تم تسجيل الدخول بنجاح", buildUserResponse(user, h.cfg.Storage.URL), token)
}

func buildUserResponse(user *models.User, storageURL string) *services.UserResponse {
	if user == nil {
		return nil
	}

	var photoURL *string
	if user.ProfilePhotoPath != nil && *user.ProfilePhotoPath != "" {
		url := storageURL + "/" + *user.ProfilePhotoPath
		photoURL = &url
	}

	return &services.UserResponse{
		ID:               user.ID,
		Name:             user.Name,
		Email:            user.Email,
		EmailVerifiedAt:  user.EmailVerifiedAt,
		Phone:            user.Phone,
		JobTitle:         user.JobTitle,
		Gender:           user.Gender,
		Country:          user.Country,
		Bio:              user.Bio,
		SocialLinks:      user.SocialLinks,
		ProfilePhotoURL:  photoURL,
		ProfilePhotoPath: user.ProfilePhotoPath,
		Status:           user.Status,
		Roles:            user.Roles,
		Permissions:      user.Permissions,
	}
}
