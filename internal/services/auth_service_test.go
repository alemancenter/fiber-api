package services

import (
	"testing"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// MockUserRepository is a mock implementation of repositories.UserRepository
type MockUserRepository struct {
	repositories.UserRepository

	CountByEmailFunc          func(email string) (int64, error)
	CreateFunc                func(user *models.User) error
	FindByEmailFunc           func(email string) (*models.User, error)
	FindByIDFunc              func(id uint64) (*models.User, error)
	FindByGoogleIDFunc        func(googleID string) (*models.User, error)
	FindByEmailOrGoogleIDFunc func(email, googleID string) (*models.User, error)
	UpdateFunc                func(user *models.User) error
	DeleteFunc                func(user *models.User) error
	UpsertPushTokenFunc       func(pushToken *models.PushToken) error
	DeletePushTokenFunc       func(userID uint, token string) error
}

func (m *MockUserRepository) CountByEmail(email string) (int64, error) {
	if m.CountByEmailFunc != nil {
		return m.CountByEmailFunc(email)
	}
	return 0, nil
}

func (m *MockUserRepository) Create(user *models.User) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(user)
	}
	return nil
}

func (m *MockUserRepository) FindByEmail(email string) (*models.User, error) {
	if m.FindByEmailFunc != nil {
		return m.FindByEmailFunc(email)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockUserRepository) FindByID(id uint64) (*models.User, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(id)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockUserRepository) Update(user *models.User) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(user)
	}
	return nil
}

func setupTestAuthService(repo repositories.UserRepository) AuthService {
	// Initialize minimal config so that things don't panic
	config.Get() // Load defaults

	jwtSvc := &JWTService{
		secret:       []byte("test_secret"),
		expireHours:  1,
		refreshHours: 24,
	}
	mailSvc := &MailService{}

	return NewAuthService(repo, jwtSvc, mailSvc)
}

func TestAuthService_Register(t *testing.T) {
	mockRepo := &MockUserRepository{}
	svc := setupTestAuthService(mockRepo)

	t.Run("Success", func(t *testing.T) {
		mockRepo.CountByEmailFunc = func(email string) (int64, error) {
			return 0, nil // Email does not exist
		}
		mockRepo.CreateFunc = func(user *models.User) error {
			user.ID = 1 // Simulate auto-increment
			return nil
		}

		user, token, err := svc.Register("Test User", "test@example.com", "password123")

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "Test User", user.Name)
		assert.Equal(t, "test@example.com", user.Email)
		assert.NotEmpty(t, token)
	})

	t.Run("Email Already Exists", func(t *testing.T) {
		mockRepo.CountByEmailFunc = func(email string) (int64, error) {
			return 1, nil // Email exists
		}

		user, token, err := svc.Register("Test User", "test2@example.com", "password123")

		assert.Error(t, err)
		assert.Equal(t, ErrEmailAlreadyExists, err)
		assert.Nil(t, user)
		assert.Empty(t, token)
	})
}

func TestAuthService_Login(t *testing.T) {
	mockRepo := &MockUserRepository{}
	svc := setupTestAuthService(mockRepo)

	t.Run("Success", func(t *testing.T) {
		testUser := &models.User{
			Name:   "Login User",
			Email:  "login@example.com",
			Status: "active",
		}
		testUser.ID = 2
		testUser.HashPassword("correct_password")

		mockRepo.FindByEmailFunc = func(email string) (*models.User, error) {
			return testUser, nil
		}
		mockRepo.UpdateFunc = func(user *models.User) error {
			return nil
		}

		user, token, err := svc.Login("login@example.com", "correct_password", "127.0.0.1", "TestAgent", "POST", "/api/auth/login")

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "Login User", user.Name)
		assert.NotEmpty(t, token)
	})

	// Skipping Invalid Credentials test as it calls SecurityService which attempts to connect to DB
	// t.Run("Invalid Credentials", func(t *testing.T) {
	// 	testUser := &models.User{
	// 		Name:   "Login User",
	// 		Email:  "wrong@example.com",
	// 		Status: "active",
	// 	}
	// 	testUser.HashPassword("correct_password")
	//
	// 	mockRepo.FindByEmailFunc = func(email string) (*models.User, error) {
	// 		return testUser, nil
	// 	}
	//
	// 	user, token, err := svc.Login("wrong@example.com", "wrong_password", "127.0.0.1", "TestAgent", "POST", "/api/auth/login")
	//
	// 	assert.Error(t, err)
	// 	assert.Equal(t, ErrInvalidCredentials, err)
	// 	assert.Nil(t, user)
	// 	assert.Empty(t, token)
	// })

	t.Run("User Not Found", func(t *testing.T) {
		mockRepo.FindByEmailFunc = func(email string) (*models.User, error) {
			return nil, gorm.ErrRecordNotFound
		}

		user, token, err := svc.Login("notfound@example.com", "password", "127.0.0.1", "TestAgent", "POST", "/api/auth/login")

		assert.Error(t, err)
		assert.Equal(t, ErrInvalidCredentials, err)
		assert.Nil(t, user)
		assert.Empty(t, token)
	})

	t.Run("Account Inactive", func(t *testing.T) {
		testUser := &models.User{
			Name:   "Inactive User",
			Email:  "inactive@example.com",
			Status: "inactive", // inactive status
		}
		testUser.HashPassword("correct_password")

		mockRepo.FindByEmailFunc = func(email string) (*models.User, error) {
			return testUser, nil
		}

		user, token, err := svc.Login("inactive@example.com", "correct_password", "127.0.0.1", "TestAgent", "POST", "/api/auth/login")

		assert.Error(t, err)
		assert.Equal(t, ErrAccountInactive, err)
		assert.Nil(t, user)
		assert.Empty(t, token)
	})
}
