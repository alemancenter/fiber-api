package services

import (
	"errors"
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

type UploadResponse struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	Type string `json:"type"`
}

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

// allowedImageExts maps MIME type → allowed file extensions for images.
// This prevents polyglot files that pass MIME detection but carry a dangerous extension.
var allowedImageExts = map[string][]string{
	"image/jpeg":               {".jpg", ".jpeg"},
	"image/png":                {".png"},
	"image/gif":                {".gif"},
	"image/webp":               {".webp"},
	"image/svg+xml":            {".svg"},
	"image/x-icon":             {".ico"},
	"image/vnd.microsoft.icon": {".ico"},
}

// allowedDocumentExts maps MIME type → allowed file extensions for documents.
var allowedDocumentExts = map[string][]string{
	"application/pdf":    {".pdf"},
	"application/msword": {".doc"},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {".docx"},
	"application/vnd.ms-excel": {".xls"},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         {".xlsx"},
	"application/vnd.ms-powerpoint":                                             {".ppt"},
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": {".pptx"},
	"text/plain": {".txt"},
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
		return nil, fmt.Errorf("failed to open uploaded file: %w", MapError(err))
	}
	defer src.Close()

	// Read first 512 bytes for MIME detection
	buf := make([]byte, 512)
	n, _ := src.Read(buf)
	mtype, err := mimetype.DetectReader(strings.NewReader(string(buf[:n])))
	if err != nil {
		return nil, fmt.Errorf("failed to detect file type: %w", MapError(err))
	}

	// Validate MIME type
	baseMime := strings.Split(mtype.String(), ";")[0]
	if !isAllowedMime(baseMime, allowed) {
		return nil, fmt.Errorf("نوع الملف غير مسموح: %s", baseMime)
	}

	// Validate that the uploaded filename's extension matches the detected MIME type.
	// This blocks polyglot files (e.g. a PHP script with a .jpg extension that passes
	// MIME detection but would execute if served directly).
	uploadedExt := strings.ToLower(filepath.Ext(header.Filename))
	if uploadedExt != "" {
		allowedExts := allowedExtensionsForMime(baseMime)
		if len(allowedExts) > 0 && !containsStr(allowedExts, uploadedExt) {
			return nil, fmt.Errorf("امتداد الملف غير مطابق لنوعه: %s", uploadedExt)
		}
	}

	// Reset reader
	if _, err := src.(io.Seeker).Seek(0, 0); err != nil {
		return nil, MapError(err)
	}

	// Generate unique filename using MIME-derived extension (not the client-supplied one)
	ext := mtype.Extension()
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(header.Filename))
	}
	filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().UnixNano(), ext)

	// relPath is always forward-slash (URL-safe); absPath uses OS separators for disk I/O
	dateDir := time.Now().Format("2006/01")
	relPath := path.Join(subdir, dateDir, filename)
	absPath := filepath.Join(s.cfg.Path, filepath.FromSlash(relPath))

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", MapError(err))
	}

	dst, err := os.Create(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", MapError(err))
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("failed to save file: %w", MapError(err))
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
		return MapError(err)
	}
	return nil
}

// GetAbsPath returns the absolute path for a relative storage path
func (s *FileService) GetAbsPath(relPath string) string {
	return filepath.Join(s.cfg.Path, relPath)
}

// SafeGetAbsPath resolves relPath within the storage root and rejects any
// path that escapes it (path traversal). Returns an error for invalid paths.
func (s *FileService) SafeGetAbsPath(relPath string) (string, error) {
	// Clean removes ".." and "." components
	cleaned := filepath.Clean(relPath)
	// Reject absolute paths supplied by the caller
	if filepath.IsAbs(cleaned) {
		return "", errors.New("absolute paths are not allowed")
	}
	storageRoot := filepath.Clean(s.cfg.Path)
	abs := filepath.Join(storageRoot, cleaned)
	// Ensure the resolved path is still under the storage root
	if !strings.HasPrefix(abs, storageRoot+string(filepath.Separator)) {
		return "", errors.New("path traversal detected")
	}
	return abs, nil
}

func isAllowedMime(mtype string, allowed []string) bool {
	for _, a := range allowed {
		if a == mtype {
			return true
		}
	}
	return false
}

func allowedExtensionsForMime(mtype string) []string {
	if exts, ok := allowedImageExts[mtype]; ok {
		return exts
	}
	if exts, ok := allowedDocumentExts[mtype]; ok {
		return exts
	}
	return nil
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
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

// FileInfoResponse is the structured payload returned by GetFileWithParent.
type FileInfoResponse struct {
	File *models.File `json:"file"`
	Item interface{}  `json:"item"`
	Type string       `json:"type"`
}

// GetFileWithParent fetches the file and its associated article or post.
// This powers the public /download/:id page.
func (s *FileService) GetFileWithParent(countryID database.CountryID, id uint64) (*FileInfoResponse, error) {
	file, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return nil, MapError(err)
	}

	db := database.DBForCountry(countryID)
	resp := &FileInfoResponse{File: file}

	if file.ArticleID != nil {
		var article models.Article
		if err := db.Preload("Subject").Preload("Semester").
			First(&article, *file.ArticleID).Error; err == nil {
			resp.Item = &article
			resp.Type = "article"
		}
	} else if file.PostID != nil {
		var post models.Post
		if err := db.Preload("Category").
			First(&post, *file.PostID).Error; err == nil {
			resp.Item = &post
			resp.Type = "post"
		}
	}

	return resp, nil
}

func (s *FileService) IncrementViewCount(countryID database.CountryID, id uint64) error {
	return ViewCounter.IncrementFileView(countryID, id)
}

func (s *FileService) CreateRecord(countryID database.CountryID, uploaded *UploadedFile, articleID *uint, postID *uint, fileName *string, fileCategory *string) (*models.File, error) {
	file := &models.File{
		FilePath:  uploaded.Path,
		FileType:  uploaded.Ext,
		FileName:  uploaded.Name,
		FileSize:  uploaded.Size,
		MimeType:  uploaded.MimeType,
		ArticleID: articleID,
		PostID:    postID,
	}

	if fileName != nil && *fileName != "" {
		file.FileName = *fileName
	}
	if fileCategory != nil && *fileCategory != "" {
		file.FileCategory = fileCategory
	}

	if err := s.repo.Create(countryID, file); err != nil {
		return nil, MapError(err)
	}

	return file, nil
}

// UpdateFileInput represents allowed file updates
type UpdateFileInput struct {
	FileName     string  `json:"file_name"`
	FileCategory *string `json:"file_category"`
	ArticleID    *uint   `json:"article_id"`
	PostID       *uint   `json:"post_id"`
}

func (s *FileService) UpdateRecord(countryID database.CountryID, id uint64, req *UpdateFileInput) (*models.File, error) {
	file, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return nil, MapError(err)
	}

	if req.FileName != "" {
		file.FileName = req.FileName
	}
	if req.FileCategory != nil {
		file.FileCategory = req.FileCategory
	}
	if req.ArticleID != nil {
		file.ArticleID = req.ArticleID
	}
	if req.PostID != nil {
		file.PostID = req.PostID
	}

	if err := s.repo.Update(countryID, file); err != nil {
		return nil, MapError(err)
	}

	return file, nil
}

func (s *FileService) DeleteRecord(countryID database.CountryID, id uint64) error {
	file, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return MapError(err)
	}

	// Delete physical file
	s.Delete(file.FilePath)

	return s.repo.Delete(countryID, file)
}
