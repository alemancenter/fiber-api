package services

import (
	"errors"
	"testing"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// MockArticleRepository is a mock implementation of repositories.ArticleRepository
type MockArticleRepository struct {
	repositories.ArticleRepository // embed to satisfy interface

	ListFunc                 func(countryID database.CountryID, pag utils.Pagination, filters *models.ArticleFilter) ([]models.Article, int64, error)
	FindByIDFunc             func(countryID database.CountryID, id uint64) (*models.Article, error)
	FindByIDWithCommentsFunc func(countryID database.CountryID, id uint64) (*models.Article, error)
	IncrementViewFunc        func(countryID database.CountryID, id uint64) error
	CreateFunc               func(countryID database.CountryID, article *models.Article) error
}

func (m *MockArticleRepository) List(countryID database.CountryID, pag utils.Pagination, filters *models.ArticleFilter) ([]models.Article, int64, error) {
	if m.ListFunc != nil {
		return m.ListFunc(countryID, pag, filters)
	}
	return nil, 0, nil
}

func (m *MockArticleRepository) FindByID(countryID database.CountryID, id uint64) (*models.Article, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(countryID, id)
	}
	return nil, nil
}

func (m *MockArticleRepository) FindByIDWithComments(countryID database.CountryID, id uint64) (*models.Article, error) {
	if m.FindByIDWithCommentsFunc != nil {
		return m.FindByIDWithCommentsFunc(countryID, id)
	}
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(countryID, id)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockArticleRepository) IncrementViewCount(countryID database.CountryID, id uint64) error {
	if m.IncrementViewFunc != nil {
		return m.IncrementViewFunc(countryID, id)
	}
	return nil
}

func (m *MockArticleRepository) Create(countryID database.CountryID, article *models.Article) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(countryID, article)
	}
	return nil
}

func TestArticleService_GetByID(t *testing.T) {
	t.Setenv("JWT_SECRET", "test_secret_key_12345678901234567890")
	t.Setenv("DB_HOST_JO", "localhost")
	t.Setenv("DB_NAME_JO", "test_db")
	t.Setenv("DB_USER_JO", "root")
	t.Setenv("APP_URL", "http://localhost")
	t.Setenv("FRONTEND_URL", "http://localhost:3000")

	mockRepo := &MockArticleRepository{}
	// FileService can be nil for this test as we don't use it in GetByID
	svc := NewArticleService(mockRepo, nil)

	t.Run("Success", func(t *testing.T) {
		expectedArticle := &models.Article{
			Title: "Test Article",
		}
		expectedArticle.ID = 1

		mockRepo.FindByIDFunc = func(countryID database.CountryID, id uint64) (*models.Article, error) {
			assert.Equal(t, uint64(1), id)
			return expectedArticle, nil
		}

		// We just mock IncrementViewCount so it doesn't panic
		mockRepo.IncrementViewFunc = func(countryID database.CountryID, id uint64) error {
			return nil
		}

		article, err := svc.GetByID(database.CountryJordan, 1)

		assert.NoError(t, err)
		assert.NotNil(t, article)
		assert.Equal(t, expectedArticle.Title, article.Title)
	})

	t.Run("NotFound", func(t *testing.T) {
		mockRepo.FindByIDFunc = func(countryID database.CountryID, id uint64) (*models.Article, error) {
			return nil, gorm.ErrRecordNotFound
		}

		article, err := svc.GetByID(database.CountryJordan, 999)

		assert.Error(t, err)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
		assert.Nil(t, article)
	})
}

func TestArticleService_CreateArticle(t *testing.T) {
	t.Setenv("JWT_SECRET", "test_secret_key_12345678901234567890")
	t.Setenv("DB_HOST_JO", "localhost")
	t.Setenv("DB_NAME_JO", "test_db")
	t.Setenv("DB_USER_JO", "root")
	t.Setenv("APP_URL", "http://localhost")
	t.Setenv("FRONTEND_URL", "http://localhost:3000")

	mockRepo := &MockArticleRepository{}
	svc := NewArticleService(mockRepo, nil)

	t.Run("Success", func(t *testing.T) {
		newArticle := &ArticleInput{
			Title: "New Article",
		}
		var authorID uint = 10

		mockRepo.CreateFunc = func(countryID database.CountryID, article *models.Article) error {
			assert.Equal(t, "New Article", article.Title)
			article.ID = 5 // Simulate DB setting ID
			return nil
		}

		// Save the original logger to restore later
		originalLogActivity := LogActivity
		defer func() { LogActivity = originalLogActivity }()

		// Mock LogActivity to prevent database calls during test
		LogActivity = func(action string, entityType string, entityID uint, userID uint) {}

		article, err := svc.CreateArticle(database.CountryJordan, newArticle, &authorID)

		assert.NoError(t, err)
		assert.NotNil(t, article)
		assert.Equal(t, uint(5), article.ID)
	})

	t.Run("DatabaseError", func(t *testing.T) {
		newArticle := &ArticleInput{
			Title: "New Article",
		}

		expectedErr := errors.New("db connection error")
		mockRepo.CreateFunc = func(countryID database.CountryID, article *models.Article) error {
			return expectedErr
		}

		article, err := svc.CreateArticle(database.CountryJordan, newArticle, nil)

		assert.Error(t, err)
		assert.Nil(t, article)
		assert.Equal(t, expectedErr, err)
	})
}
