package services

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
)

// FileService handles file operations like uploading, path mapping, and size calculations.
type FileService struct {
	cfg  config.StorageConfig
	repo repositories.FileRepository
}

// UploadedFile represents a successfully uploaded file
type UploadedFile struct {
	Path     string
	URL      string
	Name     string
	Size     int64
	MimeType string
	Ext      string
}

// AllowedImageTypes lists accepted image MIME types
var AllowedImageTypes = []string{
	"image/jpeg", "image/png", "image/gif", "image/webp", "image/svg+xml",
	"image/x-icon", "image/vnd.microsoft.icon",
}

// AllowedDocumentTypes lists accepted document MIME types
var AllowedDocumentTypes = []string{
	"application/pdf",
	"application/msword",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"application/vnd.ms-excel",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"application/vnd.ms-powerpoint",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"text/plain",
}

// MaxImageSize is 10MB
const MaxImageSize = 10 * 1024 * 1024

// MaxDocumentSize is 50MB
const MaxDocumentSize = 50 * 1024 * 1024

// NewFileService creates a new FileService
func NewFileService(repo repositories.FileRepository) *FileService {
	return &FileService{
		cfg:  config.Get().Storage,
		repo: repo,
	}
}

// UploadImage validates and saves an image file
func (s *FileService) UploadImage(header *multipart.FileHeader, subdir string) (*UploadedFile, error) {
	if header.Size > MaxImageSize {
		return nil, fmt.Errorf("حجم الصورة يتجاوز الحد المسموح (10MB)")
	}

	return s.upload(header, subdir, AllowedImageTypes)
}

// UploadDocument validates and saves a document file
func (s *FileService) UploadDocument(header *multipart.FileHeader, subdir string) (*UploadedFile, error) {
	if header.Size > MaxDocumentSize {
		return nil, fmt.Errorf("حجم الملف يتجاوز الحد المسموح (50MB)")
	}

	return s.upload(header, subdir, AllowedDocumentTypes)
}

// upload is the core upload implementation
func (s *FileService) upload(header *multipart.FileHeader, subdir string, allowed []string) (*UploadedFile, error) {
	src, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Read first 512 bytes for MIME detection
	buf := make([]byte, 512)
	n, _ := src.Read(buf)
	mtype, err := mimetype.DetectReader(strings.NewReader(string(buf[:n])))
	if err != nil {
		return nil, fmt.Errorf("failed to detect file type: %w", err)
	}

	// Validate MIME type
	if !isAllowedMime(mtype.String(), allowed) {
		return nil, fmt.Errorf("نوع الملف غير مسموح: %s", mtype.String())
	}

	// Reset reader
	if _, err := src.(io.Seeker).Seek(0, 0); err != nil {
		return nil, err
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = mtype.Extension()
	}
	filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().UnixNano(), ext)

	// relPath is always forward-slash (URL-safe); absPath uses OS separators for disk I/O
	dateDir := time.Now().Format("2006/01")
	relPath := path.Join(subdir, dateDir, filename)
	absPath := filepath.Join(s.cfg.Path, filepath.FromSlash(relPath))

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	dst, err := os.Create(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	return &UploadedFile{
		Path:     relPath,
		URL:      s.cfg.URL + "/" + relPath,
		Name:     header.Filename,
		Size:     header.Size,
		MimeType: mtype.String(),
		Ext:      ext,
	}, nil
}

// Delete removes a file from storage
func (s *FileService) Delete(relPath string) error {
	absPath := filepath.Join(s.cfg.Path, relPath)
	if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GetAbsPath returns the absolute path for a relative storage path
func (s *FileService) GetAbsPath(relPath string) string {
	return filepath.Join(s.cfg.Path, relPath)
}

func isAllowedMime(mtype string, allowed []string) bool {
	for _, a := range allowed {
		if a == mtype {
			return true
		}
	}
	return false
}

// Database Operations

func (s *FileService) List(countryID database.CountryID, fileType string, articleID string, limit, offset int) ([]models.File, int64, error) {
	return s.repo.ListPaginated(countryID, fileType, articleID, limit, offset)
}

func (s *FileService) FindByID(countryID database.CountryID, id uint64) (*models.File, error) {
	return s.repo.FindByID(countryID, id)
}

func (s *FileService) IncrementViewCount(countryID database.CountryID, id uint64) error {
	return s.repo.IncrementView(countryID, id)
}

func (s *FileService) CreateRecord(countryID database.CountryID, uploaded *UploadedFile, articleID *uint) (*models.File, error) {
	file := &models.File{
		FilePath:  uploaded.Path,
		FileType:  uploaded.Ext,
		FileName:  uploaded.Name,
		FileSize:  uploaded.Size,
		MimeType:  uploaded.MimeType,
		ArticleID: articleID,
	}

	if err := s.repo.Create(countryID, file); err != nil {
		return nil, err
	}

	return file, nil
}

// UpdateFileInput represents allowed file updates
type UpdateFileInput struct {
	FileName  string `json:"file_name"`
	ArticleID *uint  `json:"article_id"`
}

func (s *FileService) UpdateRecord(countryID database.CountryID, id uint64, req *UpdateFileInput) (*models.File, error) {
	file, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return nil, err
	}

	if req.FileName != "" {
		file.FileName = req.FileName
	}
	if req.ArticleID != nil {
		file.ArticleID = req.ArticleID
	}

	if err := s.repo.Update(countryID, file); err != nil {
		return nil, err
	}

	return file, nil
}

func (s *FileService) DeleteRecord(countryID database.CountryID, id uint64) error {
	file, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return err
	}

	// Delete physical file
	s.Delete(file.FilePath)

	return s.repo.Delete(countryID, file)
}
