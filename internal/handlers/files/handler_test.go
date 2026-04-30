package files

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

type MockFileRepository struct {
	repositories.FileRepository
	CreateFunc func(countryID database.CountryID, file *models.File) error
	DeleteFunc func(countryID database.CountryID, file *models.File) error
}

func (m *MockFileRepository) Create(countryID database.CountryID, file *models.File) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(countryID, file)
	}
	return nil
}

func (m *MockFileRepository) Delete(countryID database.CountryID, file *models.File) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(countryID, file)
	}
	return nil
}

func setupApp(t *testing.T) (*fiber.App, *MockFileRepository, string) {
	t.Setenv("JWT_SECRET", "test_secret_key_12345678901234567890")
	t.Setenv("DB_HOST_JO", "localhost")
	t.Setenv("DB_NAME_JO", "test_db")
	t.Setenv("DB_USER_JO", "root")
	t.Setenv("APP_URL", "http://localhost")
	t.Setenv("FRONTEND_URL", "http://localhost:3000")

	// Create a temp directory for uploads
	tempDir := t.TempDir()

	// Override config for storage path
	config.Get().Storage.Path = tempDir
	config.Get().Storage.URL = "http://localhost/storage"

	app := fiber.New()
	mockRepo := &MockFileRepository{}
	svc := services.NewFileService(mockRepo)
	h := New(svc)

	// Middleware to set country_id
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("country_id", database.CountryJordan)
		return c.Next()
	})

	api := app.Group("/api")
	api.Post("/dashboard/files", h.DashboardUpload)
	api.Post("/upload/image", h.UploadImage)
	api.Post("/upload/file", h.UploadDocument)

	return app, mockRepo, tempDir
}

func createMultipartForm(t *testing.T, fieldName string, fileName string, fileContent []byte, extraFields map[string]string) (*bytes.Buffer, string) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	assert.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(fileContent))
	assert.NoError(t, err)

	for key, val := range extraFields {
		_ = writer.WriteField(key, val)
	}

	err = writer.Close()
	assert.NoError(t, err)

	return body, writer.FormDataContentType()
}

func TestHandler_DashboardUpload_ImageForArticle(t *testing.T) {
	app, mockRepo, _ := setupApp(t)

	t.Run("SuccessUploadImageForArticle", func(t *testing.T) {
		// A tiny valid 1x1 GIF
		gifBytes := []byte{
			0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00,
			0x00, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44, 0x01, 0x00, 0x3b,
		}

		body, contentType := createMultipartForm(t, "file", "test.gif", gifBytes, map[string]string{
			"article_id":    "100",
			"file_category": "study_plan",
		})

		mockRepo.CreateFunc = func(countryID database.CountryID, file *models.File) error {
			assert.NotNil(t, file.ArticleID)
			assert.Equal(t, uint(100), *file.ArticleID)
			assert.Equal(t, "test.gif", file.FileName)
			assert.Equal(t, "image/gif", file.MimeType)
			assert.NotNil(t, file.FileCategory)
			assert.Equal(t, "study_plan", *file.FileCategory)
			return nil
		}

		req := httptest.NewRequest(http.MethodPost, "/api/dashboard/files", body)
		req.Header.Set("Content-Type", contentType)

		resp, err := app.Test(req)
		assert.NoError(t, err)

		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Logf("Response: %s", string(bodyBytes))

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var respBody map[string]interface{}
		_ = json.Unmarshal(bodyBytes, &respBody)
		assert.Equal(t, "تم رفع الملف بنجاح", respBody["message"])
	})
}

func TestHandler_UploadDocumentForArticle(t *testing.T) {
	app, mockRepo, _ := setupApp(t)

	t.Run("SuccessUploadDocument", func(t *testing.T) {
		// A simple text file
		txtBytes := []byte("This is a simple text document attachment.")

		body, contentType := createMultipartForm(t, "file", "attachment.txt", txtBytes, map[string]string{
			"article_id": "200",
		})

		mockRepo.CreateFunc = func(countryID database.CountryID, file *models.File) error {
			assert.NotNil(t, file.ArticleID)
			assert.Equal(t, uint(200), *file.ArticleID)
			assert.Equal(t, "attachment.txt", file.FileName)
			assert.Equal(t, "text/plain; charset=utf-8", file.MimeType)
			return nil
		}

		req := httptest.NewRequest(http.MethodPost, "/api/dashboard/files", body)
		req.Header.Set("Content-Type", contentType)

		resp, err := app.Test(req)
		assert.NoError(t, err)

		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Logf("Response: %s", string(bodyBytes))

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})
}
