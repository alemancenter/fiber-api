package services

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
)

// FileService handles file uploads and management
type FileService struct {
	cfg config.StorageConfig
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
func NewFileService() *FileService {
	return &FileService{cfg: config.Get().Storage}
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

	// Build destination path
	dateDir := time.Now().Format("2006/01")
	relPath := filepath.Join(subdir, dateDir, filename)
	absPath := filepath.Join(s.cfg.Path, relPath)

	// Create directory structure
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Save file
	dst, err := os.Create(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Normalize path separators for URL
	urlPath := strings.ReplaceAll(relPath, "\\", "/")

	return &UploadedFile{
		Path:     relPath,
		URL:      s.cfg.URL + "/" + urlPath,
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
