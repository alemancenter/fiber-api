package repositories

import (
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"gorm.io/gorm"
)

type CommentRepository interface {
	GetDB(dbCode string) *gorm.DB
	GetDBByCountry(countryID database.CountryID) *gorm.DB
	ListPaginated(dbCode string, commentableType string, commentableID string, limit, offset int) ([]models.Comment, int64, error)
	CreateComment(dbCode string, comment *models.Comment) error
	DeleteComment(dbCode string, id uint64) error
	UpsertReaction(countryID database.CountryID, reaction *models.Reaction) error
	DeleteReaction(countryID database.CountryID, commentID uint64, userID uint) error
	GetReactions(countryID database.CountryID, commentID uint64) ([]models.Reaction, error)
}

type commentRepository struct{}

func NewCommentRepository() CommentRepository {
	return &commentRepository{}
}

func (r *commentRepository) GetDB(dbCode string) *gorm.DB {
	return database.GetManager().GetByCode(dbCode)
}

func (r *commentRepository) GetDBByCountry(countryID database.CountryID) *gorm.DB {
	return database.DBForCountry(countryID)
}

func (r *commentRepository) ListPaginated(dbCode string, commentableType string, commentableID string, limit, offset int) ([]models.Comment, int64, error) {
	db := r.GetDB(dbCode)
	var commentList []models.Comment
	var total int64

	query := db.Model(&models.Comment{}).Preload("User").Where("`database` = ?", dbCode)

	if commentableType != "" {
		query = query.Where("commentable_type = ?", commentableType)
	}
	if commentableID != "" {
		query = query.Where("commentable_id = ?", commentableID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&commentList).Error
	return commentList, total, err
}

func (r *commentRepository) CreateComment(dbCode string, comment *models.Comment) error {
	db := r.GetDB(dbCode)
	return db.Create(comment).Error
}

func (r *commentRepository) DeleteComment(dbCode string, id uint64) error {
	db := r.GetDB(dbCode)
	return db.Delete(&models.Comment{}, id).Error
}

func (r *commentRepository) UpsertReaction(countryID database.CountryID, reaction *models.Reaction) error {
	db := r.GetDBByCountry(countryID)
	return db.Where(models.Reaction{CommentID: reaction.CommentID, UserID: reaction.UserID}).
		Assign(*reaction).
		FirstOrCreate(reaction).Error
}

func (r *commentRepository) DeleteReaction(countryID database.CountryID, commentID uint64, userID uint) error {
	db := r.GetDBByCountry(countryID)
	return db.Where("comment_id = ? AND user_id = ?", commentID, userID).Delete(&models.Reaction{}).Error
}

func (r *commentRepository) GetReactions(countryID database.CountryID, commentID uint64) ([]models.Reaction, error) {
	db := r.GetDBByCountry(countryID)
	var reactions []models.Reaction
	err := db.Preload("User").Where("comment_id = ?", commentID).Find(&reactions).Error
	return reactions, err
}
