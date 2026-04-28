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
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
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

// AuthService handles business logic for authentication
type AuthService interface {
	Register(name, email, password string) (*models.User, string, error)
	Login(email, password, ip, userAgent, method, path string) (*models.User, string, error)
	Logout(tokenStr string, user *models.User) error
	UpdateProfile(user *models.User, updates map[string]interface{}) (*models.User, error)
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
	_ = s.repo.Update(user, map[string]interface{}{"last_activity": now})

	return user, token, nil
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

func (s *authService) UpdateProfile(user *models.User, updates map[string]interface{}) (*models.User, error) {
	if err := s.repo.Update(user, updates); err != nil {
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
	go s.mailSvc.SendPasswordResetEmail(user.Email, user.Name, resetURL)

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

	if err := s.repo.Update(user, map[string]interface{}{"password": user.Password}); err != nil {
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
	return s.repo.Update(user, map[string]interface{}{"email_verified_at": now})
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
		if user.GoogleID == nil {
			_ = s.repo.Update(user, map[string]interface{}{"google_id": info.ID})
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
	_ = s.mailSvc.SendVerificationEmail(user.Email, user.Name, verifyURL)
}
