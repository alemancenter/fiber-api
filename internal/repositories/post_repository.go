package repositories

import (
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"gorm.io/gorm"
)

type PostRepository interface {
	GetDB(countryID database.CountryID) *gorm.DB
	ListPaginated(countryID database.CountryID, catID string, search string, featured string, limit, offset int) ([]models.Post, int64, error)
	FindByID(countryID database.CountryID, id uint64) (*models.Post, error)
	IncrementView(countryID database.CountryID, id uint64) error
	Create(countryID database.CountryID, post *models.Post) error
	Update(countryID database.CountryID, post *models.Post) error
	Delete(countryID database.CountryID, id uint64) error
}

type postRepository struct{}

func NewPostRepository() PostRepository {
	return &postRepository{}
}

func (r *postRepository) GetDB(countryID database.CountryID) *gorm.DB {
	return database.DBForCountry(countryID)
}

func (r *postRepository) ListPaginated(countryID database.CountryID, catID string, search string, featured string, limit, offset int) ([]models.Post, int64, error) {
	db := r.GetDB(countryID)
	var postList []models.Post
	var total int64

	query := db.Model(&models.Post{}).Preload("Category").Preload("Author").
		Where("is_active = ?", true)

	if catID != "" {
		query = query.Where("category_id = ?", catID)
	}
	if search != "" {
		// Search title only — content is a large TEXT column and scanning it causes
		// a full-table scan even with indexes. Add a FULLTEXT index on content and
		// switch to MATCH() AGAINST() if full-content search is required.
		query = query.Where("title LIKE ?", "%"+search+"%")
	}
	if featured == "1" {
		query = query.Where("is_featured = ?", true)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&postList).Error
	return postList, total, err
}

func (r *postRepository) FindByID(countryID database.CountryID, id uint64) (*models.Post, error) {
	db := r.GetDB(countryID)
	var post models.Post
	err := db.Preload("Category").Preload("Author").Preload("Comments.User").
		Preload("KeywordsRel").
		Where("id = ? AND is_active = ?", id, true).
		First(&post).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *postRepository) IncrementView(countryID database.CountryID, id uint64) error {
	db := r.GetDB(countryID)
	return db.Model(&models.Post{}).Where("id = ?", id).
		UpdateColumn("views", gorm.Expr("views + 1")).Error
}

func (r *postRepository) Create(countryID database.CountryID, post *models.Post) error {
	db := r.GetDB(countryID)
	return db.Create(post).Error
}

func (r *postRepository) Update(countryID database.CountryID, post *models.Post) error {
	db := r.GetDB(countryID)
	return db.Save(post).Error
}

func (r *postRepository) Delete(countryID database.CountryID, id uint64) error {
	db := r.GetDB(countryID)
	return db.Delete(&models.Post{}, id).Error
}
