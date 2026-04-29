package services

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/pkg/logger"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrEmailAlreadyExists = errors.New("البريد الإلكتروني مستخدم بالفعل")
	ErrInvalidCredentials = errors.New("بيانات الاعتماد غير صحيحة")
	ErrAccountInactive    = errors.New("الحساب غير نشط أو محظور")
	ErrInvalidResetToken  = errors.New("رمز إعادة التعيين غير صالح أو منتهي الصلاحية")
	ErrInvalidVerifyToken = errors.New("رابط التحقق غير صالح")
	ErrAlreadyVerified    = errors.New("البريد الإلكتروني مُحقق بالفعل")
)

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
}

type UpdateProfileInput struct {
	Name             string  `json:"name"`
	Phone            *string `json:"phone"`
	JobTitle         *string `json:"job_title"`
	Gender           *string `json:"gender"`
	Country          *string `json:"country"`
	Bio              *string `json:"bio"`
	SocialLinks      *string `json:"social_links"`
	ProfilePhotoPath *string `json:"profile_photo_path"`
}

// AuthService handles business logic for authentication
type AuthService interface {
	Register(name, email, password string) (*models.User, string, error)
	Login(email, password, ip, userAgent, method, path string) (*models.User, string, error)
	Logout(tokenStr string, user *models.User) error
	RefreshToken(refreshTokenStr string) (accessToken, newRefreshToken string, err error)
	GenerateRefreshTokenForUser(userID uint, email string) (string, error)
	UpdateProfile(user *models.User, req *UpdateProfileInput) (*models.User, error)
	ForgotPassword(email string) error
	ResetPassword(token, email, newPassword string) error
	VerifyEmail(id string, hash string) error
	ResendVerification(user *models.User) error
	DeleteAccount(user *models.User, password string) error
	GetGoogleOAuthConfig() *oauth2.Config
	LoginOrRegisterGoogleUser(info *GoogleUserInfo) (*models.User, string, error)
	UpsertPushToken(userID uint, token, platform string) error
	DeletePushToken(userID uint, token string) error
}

type UserResponse struct {
	ID               uint                `json:"id"`
	Name             string              `json:"name"`
	Email            string              `json:"email"`
	EmailVerifiedAt  *time.Time          `json:"email_verified_at"`
	Phone            *string             `json:"phone"`
	JobTitle         *string             `json:"job_title"`
	Gender           *string             `json:"gender"`
	Country          *string             `json:"country"`
	Bio              *string             `json:"bio"`
	SocialLinks      *string             `json:"social_links"`
	ProfilePhotoURL  *string             `json:"profile_photo_url"`
	ProfilePhotoPath *string             `json:"profile_photo_path"`
	Status           string              `json:"status"`
	Roles            []models.Role       `json:"roles"`
	Permissions      []models.Permission `json:"permissions"`
}

type RegisterResponse struct {
	Success bool          `json:"success"`
	Message string        `json:"message"`
	Token   string        `json:"token"`
	User    *UserResponse `json:"user"`
}

type authService struct {
	repo    repositories.UserRepository
	jwtSvc  *JWTService
	mailSvc *MailService
	cfg     *config.Config
}

// NewAuthService creates a new AuthService
func NewAuthService(repo repositories.UserRepository, jwtSvc *JWTService, mailSvc *MailService) AuthService {
	return &authService{
		repo:    repo,
		jwtSvc:  jwtSvc,
		mailSvc: mailSvc,
		cfg:     config.Get(),
	}
}

func (s *authService) Register(name, email, password string) (*models.User, string, error) {
	count, err := s.repo.CountByEmail(email)
	if err != nil {
		return nil, "", err
	}
	if count > 0 {
		return nil, "", ErrEmailAlreadyExists
	}

	user := models.User{
		Name:   name,
		Email:  email,
		Status: "active",
	}

	if err := user.HashPassword(password); err != nil {
		return nil, "", err
	}

	if err := s.repo.Create(&user); err != nil {
		return nil, "", err
	}

	// Send verification email (async)
	go s.sendVerificationEmail(&user)

	token, err := s.jwtSvc.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, "", err
	}

	return &user, token, nil
}

func (s *authService) Login(email, password, ip, userAgent, method, path string) (*models.User, string, error) {
	user, err := s.repo.FindByEmail(email)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, "", ErrInvalidCredentials
		}
		return nil, "", err
	}

	if !user.CheckPassword(password) {
		// Create security service directly since we don't inject it here yet to avoid circular deps
		secRepo := repositories.NewSecurityRepository()
		secSvc := NewSecurityService(secRepo)
		secSvc.LogEvent(
			ip,
			models.EventLoginFailed,
			"فشل تسجيل الدخول للبريد: "+email,
			models.SeverityWarning,
			WithRoute(path),
			WithMethod(method),
			WithUserAgent(userAgent),
		)
		return nil, "", ErrInvalidCredentials
	}

	if !user.IsActive() {
		return nil, "", ErrAccountInactive
	}

	token, err := s.jwtSvc.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, "", err
	}

	// Update last activity
	now := time.Now()
	user.LastActivity = &now
	_ = s.repo.Update(user)

	return user, token, nil
}

func (s *authService) GenerateRefreshTokenForUser(userID uint, email string) (string, error) {
	return s.jwtSvc.GenerateRefreshToken(userID, email)
}

func (s *authService) Logout(tokenStr string, user *models.User) error {
	rdb := database.Redis()
	ctx := context.Background()

	if tokenStr != "" {
		hash := sha256.Sum256([]byte(tokenStr))
		key := rdb.Key("blacklist", fmt.Sprintf("%x", hash))
		_ = rdb.Set(ctx, key, "1", time.Duration(s.cfg.JWT.ExpireHours)*time.Hour)
	}

	// Evict user cache
	if user != nil {
		_ = rdb.Del(ctx, rdb.Key("user", fmt.Sprintf("%d", user.ID)))
	}

	return nil
}

func (s *authService) RefreshToken(refreshTokenStr string) (string, string, error) {
	claims, err := s.jwtSvc.ValidateRefreshToken(refreshTokenStr)
	if err != nil {
		return "", "", ErrInvalidCredentials
	}

	user, err := s.repo.FindByID(uint64(claims.UserID))
	if err != nil {
		return "", "", ErrInvalidCredentials
	}
	if !user.IsActive() {
		return "", "", ErrAccountInactive
	}

	accessToken, err := s.jwtSvc.GenerateToken(user.ID, user.Email)
	if err != nil {
		return "", "", err
	}
	newRefresh, err := s.jwtSvc.GenerateRefreshToken(user.ID, user.Email)
	if err != nil {
		return "", "", err
	}
	return accessToken, newRefresh, nil
}

func (s *authService) UpdateProfile(user *models.User, req *UpdateProfileInput) (*models.User, error) {
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Phone != nil {
		user.Phone = req.Phone
	}
	if req.JobTitle != nil {
		user.JobTitle = req.JobTitle
	}
	if req.Gender != nil {
		user.Gender = req.Gender
	}
	if req.Country != nil {
		user.Country = req.Country
	}
	if req.Bio != nil {
		user.Bio = req.Bio
	}
	if req.SocialLinks != nil {
		user.SocialLinks = req.SocialLinks
	}
	if req.ProfilePhotoPath != nil {
		user.ProfilePhotoPath = req.ProfilePhotoPath
	}

	if err := s.repo.Update(user); err != nil {
		return nil, err
	}

	// Reload user
	updatedUser, err := s.repo.FindByID(uint64(user.ID))
	if err != nil {
		return nil, err
	}

	return updatedUser, nil
}

func (s *authService) ForgotPassword(email string) error {
	user, err := s.repo.FindByEmail(email)
	if err != nil {
		// Return nil even if email not found (prevent enumeration)
		return nil
	}

	// Generate reset token
	// Assuming generateSecureToken is available in utils or similar.
	// For now we'll implement a simple one here if not available globally.
	// We'll use the JWTService as a workaround to get a secure random string or generate it directly.
	token := s.jwtSvc.GenerateRandomString(32) // We will need to add this method to JWTService or use a helper
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))

	// Store in Redis with 60-minute expiry
	rdb := database.Redis()
	ctx := context.Background()
	key := rdb.Key("password_reset", hash)
	_ = rdb.Set(ctx, key, fmt.Sprintf("%d", user.ID), 60*time.Minute)

	// Send reset email
	resetURL := fmt.Sprintf("%s/reset-password?token=%s&email=%s",
		s.cfg.Frontend.URL, token, email)
	go func() {
		if err := s.mailSvc.SendPasswordResetEmail(user.Email, user.Name, resetURL); err != nil {
			logger.Error("failed to send password reset email",
				zap.Uint("user_id", user.ID),
				zap.String("email", user.Email),
				zap.Error(err),
			)
		}
	}()

	return nil
}

func (s *authService) ResetPassword(token, email, newPassword string) error {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	rdb := database.Redis()
	ctx := context.Background()
	key := rdb.Key("password_reset", hash)

	userIDStr, err := rdb.Get(ctx, key)
	if err != nil {
		return ErrInvalidResetToken
	}

	user, err := s.repo.FindByEmail(email)
	if err != nil || fmt.Sprintf("%d", user.ID) != userIDStr {
		return ErrInvalidCredentials
	}

	if err := user.HashPassword(newPassword); err != nil {
		return err
	}

	if err := s.repo.Update(user); err != nil {
		return err
	}

	// Delete used token
	_ = rdb.Del(ctx, key)

	return nil
}

func (s *authService) VerifyEmail(id string, hash string) error {
	// Parse ID
	var userID uint64
	fmt.Sscanf(id, "%d", &userID)

	user, err := s.repo.FindByID(userID)
	if err != nil {
		return err
	}

	expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(user.Email)))
	if hash != expectedHash {
		return ErrInvalidVerifyToken
	}

	if user.IsVerified() {
		return ErrAlreadyVerified
	}

	now := time.Now()
	user.EmailVerifiedAt = &now
	return s.repo.Update(user)
}

func (s *authService) ResendVerification(user *models.User) error {
	if user.IsVerified() {
		return ErrAlreadyVerified
	}

	go s.sendVerificationEmail(user)
	return nil
}

func (s *authService) DeleteAccount(user *models.User, password string) error {
	if !user.CheckPassword(password) {
		return ErrInvalidCredentials
	}

	return s.repo.Delete(user)
}

func (s *authService) GetGoogleOAuthConfig() *oauth2.Config {
	cfg := s.cfg.Google
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func (s *authService) LoginOrRegisterGoogleUser(info *GoogleUserInfo) (*models.User, string, error) {
	user, err := s.repo.FindByEmailOrGoogleID(info.Email, info.ID)

	if err == gorm.ErrRecordNotFound {
		// Register new user
		now := time.Now()
		user = &models.User{
			Name:            info.Name,
			Email:           info.Email,
			GoogleID:        &info.ID,
			EmailVerifiedAt: &now,
			Status:          "active",
		}
		_ = user.HashPassword(s.jwtSvc.GenerateRandomString(32))
		if err := s.repo.Create(user); err != nil {
			return nil, "", err
		}
	} else if err != nil {
		return nil, "", err
	} else {
		// Update Google ID if not set
		if user.GoogleID == nil || *user.GoogleID != info.ID {
			user.GoogleID = &info.ID
			_ = s.repo.Update(user)
		}
	}

	token, err := s.jwtSvc.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (s *authService) UpsertPushToken(userID uint, token, platform string) error {
	pushToken := &models.PushToken{
		UserID:   userID,
		Token:    token,
		Platform: platform,
	}
	return s.repo.UpsertPushToken(pushToken)
}

func (s *authService) DeletePushToken(userID uint, token string) error {
	return s.repo.DeletePushToken(userID, token)
}

// --- Helper functions ---

func (s *authService) sendVerificationEmail(user *models.User) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(user.Email)))
	verifyURL := fmt.Sprintf("%s/api/auth/email/verify/%d/%s",
		s.cfg.App.URL, user.ID, hash)
	if err := s.mailSvc.SendVerificationEmail(user.Email, user.Name, verifyURL); err != nil {
		logger.Error("failed to send verification email",
			zap.Uint("user_id", user.ID),
			zap.String("email", user.Email),
			zap.Error(err),
		)
	}
}
