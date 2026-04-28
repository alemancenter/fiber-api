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
	Update(countryID database.CountryID, id uint64, req *UpdatePostRequest) (*models.Post, error)
	Delete(countryID database.CountryID, id uint64) error
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
	return s.repo.ListPaginated(countryID, catID, search, featured, limit, offset)
}

func (s *postService) GetByID(countryID database.CountryID, id uint64) (*models.Post, error) {
	return s.repo.FindByID(countryID, id)
}

func (s *postService) IncrementView(countryID database.CountryID, id uint64) error {
	return s.repo.IncrementView(countryID, id)
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
		return nil, err
	}

	return post, nil
}

func (s *postService) Update(countryID database.CountryID, id uint64, req *UpdatePostRequest) (*models.Post, error) {
	post, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.CategoryID != nil {
		updates["category_id"] = req.CategoryID
	}
	if req.Title != "" {
		updates["title"] = utils.SanitizeInput(req.Title)
		updates["slug"] = utils.GenerateSlug(req.Title)
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsFeatured != nil {
		updates["is_featured"] = *req.IsFeatured
	}
	if req.Keywords != "" {
		updates["keywords"] = req.Keywords
	}
	if req.MetaDescription != "" {
		updates["meta_description"] = req.MetaDescription
	}

	if len(updates) > 0 {
		if err := s.repo.Update(countryID, post, updates); err != nil {
			return nil, err
		}
	}

	return post, nil
}

func (s *postService) Delete(countryID database.CountryID, id uint64) error {
	return s.repo.Delete(countryID, id)
}
