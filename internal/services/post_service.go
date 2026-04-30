package services

import (
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/utils"
)

type PostService interface {
	List(countryID database.CountryID, filter *models.PostFilter, limit, offset int) ([]models.Post, int64, error)
	GetByID(countryID database.CountryID, id uint64) (*models.Post, error)
	IncrementView(countryID database.CountryID, id uint64) error
	Create(countryID database.CountryID, countryCode string, userID *uint, req *CreatePostRequest, imagePath string) (*models.Post, error)
	Update(countryID database.CountryID, id uint64, req *UpdatePostRequest, callerID uint, callerIsAdmin bool) (*models.Post, error)
	Delete(countryID database.CountryID, id uint64, callerID uint, callerIsAdmin bool) error
}

type CreatePostRequest struct {
	CategoryID      uint   `json:"category_id" form:"category_id" validate:"required"`
	Title           string `json:"title" form:"title" validate:"required,min=3,max=500"`
	Content         string `json:"content" form:"content" validate:"required"`
	IsActive        bool   `json:"is_active" form:"is_active"`
	IsFeatured      bool   `json:"is_featured" form:"is_featured"`
	Keywords        string `json:"keywords" form:"keywords"`
	MetaDescription string `json:"meta_description" form:"meta_description" validate:"omitempty,max=500"`
}

type UpdatePostRequest struct {
	CategoryID      *uint   `json:"category_id" form:"category_id"`
	Title           string  `json:"title" form:"title"`
	Content         string  `json:"content" form:"content"`
	IsActive        *bool   `json:"is_active" form:"is_active"`
	IsFeatured      *bool   `json:"is_featured" form:"is_featured"`
	Keywords        string  `json:"keywords" form:"keywords"`
	MetaDescription string  `json:"meta_description" form:"meta_description"`
	ImagePath       *string `json:"image_path" form:"image_path"`
}

type postService struct {
	repo  repositories.PostRepository
	cache CacheService
}

func NewPostService(repo repositories.PostRepository, cache CacheService) PostService {
	return &postService{repo: repo, cache: cache}
}

func (s *postService) List(countryID database.CountryID, filter *models.PostFilter, limit, offset int) ([]models.Post, int64, error) {
	cacheKey := utils.CacheKey("posts:list", countryID, limit, offset, filter)

	var cached struct {
		Posts []models.Post `json:"posts"`
		Total int64         `json:"total"`
	}

	if s.cache != nil && s.cache.Get(cacheKey, &cached) {
		return cached.Posts, cached.Total, nil
	}

	posts, total, err := s.repo.ListPaginated(countryID, filter, limit, offset)
	if err != nil {
		return nil, 0, MapError(err)
	}

	if s.cache != nil {
		_ = s.cache.Set(cacheKey, struct {
			Posts []models.Post `json:"posts"`
			Total int64         `json:"total"`
		}{
			Posts: posts,
			Total: total,
		}, 5*time.Minute)
	}

	return posts, total, nil
}

func (s *postService) GetByID(countryID database.CountryID, id uint64) (*models.Post, error) {
	post, err := s.repo.FindByID(countryID, id)
	return post, MapError(err)
}

func (s *postService) IncrementView(countryID database.CountryID, id uint64) error {
	return ViewCounter.IncrementPostView(countryID, id)
}

func (s *postService) uniqueSlug(countryID database.CountryID, base string, excludeID uint64) string {
	candidate := base
	for i := 1; ; i++ {
		if !s.repo.ExistsBySlug(countryID, candidate, excludeID) {
			return candidate
		}
		candidate = utils.NumberedSlug(base, i)
	}
}

func (s *postService) Create(countryID database.CountryID, countryCode string, userID *uint, req *CreatePostRequest, imagePath string) (*models.Post, error) {
	slug := s.uniqueSlug(countryID, utils.GenerateSlug(req.Title), 0)
	post := &models.Post{
		Title:      utils.SanitizeInput(req.Title),
		Content:    req.Content,
		Slug:       slug,
		IsActive:   req.IsActive,
		IsFeatured: req.IsFeatured,
		Country:    countryCode,
	}

	if req.CategoryID > 0 {
		post.CategoryID = &req.CategoryID
	}
	if req.Keywords != "" {
		post.Keywords = &req.Keywords
	}
	if req.MetaDescription != "" {
		post.MetaDescription = &req.MetaDescription
	}
	if userID != nil {
		post.AuthorID = userID
	}
	if imagePath != "" {
		post.Image = &imagePath
	}

	if err := s.repo.Create(countryID, post); err != nil {
		return nil, MapError(err)
	}

	if s.cache != nil {
		_ = s.cache.DeletePattern("posts:list:*")
		_ = s.cache.DeletePattern("home:*")
	}

	return post, nil
}

func (s *postService) Update(countryID database.CountryID, id uint64, req *UpdatePostRequest, callerID uint, callerIsAdmin bool) (*models.Post, error) {
	post, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return nil, MapError(err)
	}

	if !callerIsAdmin && callerID > 0 && post.AuthorID != nil && *post.AuthorID != callerID {
		return nil, ErrForbidden
	}

	if req.CategoryID != nil {
		post.CategoryID = req.CategoryID
	}
	if req.Title != "" {
		post.Title = utils.SanitizeInput(req.Title)
		post.Slug = s.uniqueSlug(countryID, utils.GenerateSlug(req.Title), id)
	}
	if req.Content != "" {
		post.Content = req.Content
	}
	if req.IsActive != nil {
		post.IsActive = *req.IsActive
	}
	if req.IsFeatured != nil {
		post.IsFeatured = *req.IsFeatured
	}
	if req.Keywords != "" {
		post.Keywords = &req.Keywords
	}
	if req.MetaDescription != "" {
		post.MetaDescription = &req.MetaDescription
	}
	if req.ImagePath != nil {
		post.Image = req.ImagePath
	}

	if err := s.repo.Update(countryID, post); err != nil {
		return nil, MapError(err)
	}

	if s.cache != nil {
		_ = s.cache.DeletePattern("posts:list:*")
		_ = s.cache.DeletePattern("home:*")
	}

	return post, nil
}

func (s *postService) Delete(countryID database.CountryID, id uint64, callerID uint, callerIsAdmin bool) error {
	post, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return MapError(err)
	}

	if !callerIsAdmin && callerID > 0 && post.AuthorID != nil && *post.AuthorID != callerID {
		return ErrForbidden
	}

	err = s.repo.Delete(countryID, id)
	if err == nil && s.cache != nil {
		_ = s.cache.DeletePattern("posts:list:*")
		_ = s.cache.DeletePattern("home:*")
	}
	return MapError(err)
}
