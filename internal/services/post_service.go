package services

import (
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/utils"
)

type PostService interface {
	List(countryID database.CountryID, catID string, search string, featured string, limit, offset int) ([]models.Post, int64, error)
	GetByID(countryID database.CountryID, id uint64) (*models.Post, error)
	IncrementView(countryID database.CountryID, id uint64) error
	Create(countryID database.CountryID, countryCode string, userID *uint, req *CreatePostRequest, imagePath string) (*models.Post, error)
	Update(countryID database.CountryID, id uint64, req *UpdatePostRequest, callerID uint, callerIsAdmin bool) (*models.Post, error)
	Delete(countryID database.CountryID, id uint64, callerID uint, callerIsAdmin bool) error
}

type CreatePostRequest struct {
	CategoryID      uint   `json:"category_id"`
	Title           string `json:"title" validate:"required,min=3,max=500"`
	Content         string `json:"content" validate:"required"`
	IsActive        bool   `json:"is_active"`
	IsFeatured      bool   `json:"is_featured"`
	Keywords        string `json:"keywords"`
	MetaDescription string `json:"meta_description" validate:"omitempty,max=500"`
}

type UpdatePostRequest struct {
	CategoryID      *uint  `json:"category_id"`
	Title           string `json:"title"`
	Content         string `json:"content"`
	IsActive        *bool  `json:"is_active"`
	IsFeatured      *bool  `json:"is_featured"`
	Keywords        string `json:"keywords"`
	MetaDescription string `json:"meta_description"`
}

type postService struct {
	repo repositories.PostRepository
}

func NewPostService(repo repositories.PostRepository) PostService {
	return &postService{repo: repo}
}

func (s *postService) List(countryID database.CountryID, catID string, search string, featured string, limit, offset int) ([]models.Post, int64, error) {
	posts, total, err := s.repo.ListPaginated(countryID, catID, search, featured, limit, offset)
	return posts, total, MapError(err)
}

func (s *postService) GetByID(countryID database.CountryID, id uint64) (*models.Post, error) {
	post, err := s.repo.FindByID(countryID, id)
	return post, MapError(err)
}

func (s *postService) IncrementView(countryID database.CountryID, id uint64) error {
	return ViewCounter.IncrementPostView(countryID, id)
}

func (s *postService) Create(countryID database.CountryID, countryCode string, userID *uint, req *CreatePostRequest, imagePath string) (*models.Post, error) {
	slug := utils.GenerateSlug(req.Title)
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
		post.Slug = utils.GenerateSlug(req.Title)
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

	if err := s.repo.Update(countryID, post); err != nil {
		return nil, MapError(err)
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

	return s.repo.Delete(countryID, id)
}
