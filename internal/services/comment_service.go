package services

import (
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/utils"
)

type CommentService interface {
	List(dbCode string, commentableType string, commentableID string, limit, offset int) ([]models.Comment, int64, error)
	Create(dbCode string, userID uint, req *CreateCommentRequest) (*models.Comment, error)
	Delete(dbCode string, id uint64) error
	CreateReaction(countryID database.CountryID, userID uint, req *ReactionRequest) (*models.Reaction, error)
	DeleteReaction(countryID database.CountryID, commentID uint64, userID uint) error
	GetReactions(countryID database.CountryID, commentID uint64) ([]models.Reaction, error)
}

type CreateCommentRequest struct {
	Body            string `json:"body" validate:"required,min=1"`
	CommentableID   uint   `json:"commentable_id" validate:"required"`
	CommentableType string `json:"commentable_type" validate:"required"`
}

type ReactionRequest struct {
	CommentID uint   `json:"comment_id" validate:"required"`
	Emoji     string `json:"emoji" validate:"required,max=20"`
}

type commentService struct {
	repo repositories.CommentRepository
}

func NewCommentService(repo repositories.CommentRepository) CommentService {
	return &commentService{repo: repo}
}

func (s *commentService) List(dbCode string, commentableType string, commentableID string, limit, offset int) ([]models.Comment, int64, error) {
	return s.repo.ListPaginated(dbCode, commentableType, commentableID, limit, offset)
}

func (s *commentService) Create(dbCode string, userID uint, req *CreateCommentRequest) (*models.Comment, error) {
	comment := &models.Comment{
		Body:            utils.SanitizeInput(req.Body),
		UserID:          userID,
		CommentableID:   req.CommentableID,
		CommentableType: req.CommentableType,
		Database:        dbCode,
	}

	if err := s.repo.CreateComment(dbCode, comment); err != nil {
		return nil, err
	}

	return comment, nil
}

func (s *commentService) Delete(dbCode string, id uint64) error {
	return s.repo.DeleteComment(dbCode, id)
}

func (s *commentService) CreateReaction(countryID database.CountryID, userID uint, req *ReactionRequest) (*models.Reaction, error) {
	reaction := &models.Reaction{
		CommentID: req.CommentID,
		UserID:    userID,
		Emoji:     req.Emoji,
	}

	if err := s.repo.UpsertReaction(countryID, reaction); err != nil {
		return nil, err
	}

	return reaction, nil
}

func (s *commentService) DeleteReaction(countryID database.CountryID, commentID uint64, userID uint) error {
	return s.repo.DeleteReaction(countryID, commentID, userID)
}

func (s *commentService) GetReactions(countryID database.CountryID, commentID uint64) ([]models.Reaction, error) {
	return s.repo.GetReactions(countryID, commentID)
}
