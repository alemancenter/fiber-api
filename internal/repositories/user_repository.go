package repositories

import (
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"gorm.io/gorm"
)

// UserRepository handles database operations for users
type UserRepository interface {
	GetDB() *gorm.DB
	CountByEmail(email string) (int64, error)
	List(search, status, role string, limit, offset int) ([]models.User, int64, error)
	Search(query string, limit int) ([]models.User, error)
	Create(user *models.User) error
	FindByEmail(email string) (*models.User, error)
	FindByID(id uint64) (*models.User, error)
	FindByGoogleID(googleID string) (*models.User, error)
	FindByEmailOrGoogleID(email string, googleID string) (*models.User, error)
	Update(user *models.User) error
	Delete(user *models.User) error
	BulkDelete(ids []uint) error
	UpdateStatus(ids []uint, status string) error
	UpsertPushToken(pushToken *models.PushToken) error
	DeletePushToken(userID uint, token string) error
}

type userRepository struct{}

// NewUserRepository creates a new UserRepository
func NewUserRepository() UserRepository {
	return &userRepository{}
}

func (r *userRepository) GetDB() *gorm.DB {
	return database.DB()
}

func (r *userRepository) CountByEmail(email string) (int64, error) {
	db := r.GetDB()
	var count int64
	err := db.Model(&models.User{}).Where("email = ?", email).Count(&count).Error
	return count, err
}

func (r *userRepository) List(search, status, role string, limit, offset int) ([]models.User, int64, error) {
	db := r.GetDB()
	var users []models.User
	var total int64

	query := db.Model(&models.User{}).Preload("Roles")

	if search != "" {
		query = query.Where("name LIKE ? OR email LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if role != "" {
		query = query.Joins("JOIN model_has_roles ON model_has_roles.model_id = users.id").
			Joins("JOIN roles ON roles.id = model_has_roles.role_id").
			Where("roles.name = ?", role)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&users).Error
	return users, total, err
}

func (r *userRepository) Search(query string, limit int) ([]models.User, error) {
	db := r.GetDB()
	var users []models.User
	err := db.Select("id, name, email, profile_photo_path").
		Where("name LIKE ? OR email LIKE ?", "%"+query+"%", "%"+query+"%").
		Limit(limit).Find(&users).Error
	return users, err
}

func (r *userRepository) Create(user *models.User) error {
	db := r.GetDB()
	return db.Create(user).Error
}

func (r *userRepository) FindByEmail(email string) (*models.User, error) {
	db := r.GetDB()
	var user models.User
	err := db.Preload("Roles.Permissions").Preload("Permissions").
		Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByID(id uint64) (*models.User, error) {
	db := r.GetDB()
	var user models.User
	err := db.Preload("Roles.Permissions").Preload("Permissions").
		Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByGoogleID(googleID string) (*models.User, error) {
	db := r.GetDB()
	var user models.User
	err := db.Preload("Roles.Permissions").Preload("Permissions").
		Where("google_id = ?", googleID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmailOrGoogleID(email string, googleID string) (*models.User, error) {
	db := r.GetDB()
	var user models.User
	err := db.Preload("Roles.Permissions").Preload("Permissions").
		Where("email = ? OR google_id = ?", email, googleID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Update(user *models.User) error {
	db := r.GetDB()
	return db.Save(user).Error
}

func (r *userRepository) Delete(user *models.User) error {
	db := r.GetDB()
	return db.Delete(user).Error
}

func (r *userRepository) BulkDelete(ids []uint) error {
	db := r.GetDB()
	return db.Where("id IN ?", ids).Delete(&models.User{}).Error
}

func (r *userRepository) UpdateStatus(ids []uint, status string) error {
	db := r.GetDB()
	return db.Model(&models.User{}).Where("id IN ?", ids).Update("status", status).Error
}

func (r *userRepository) UpsertPushToken(pushToken *models.PushToken) error {
	db := r.GetDB()
	return db.Where(models.PushToken{Token: pushToken.Token}).Assign(pushToken).FirstOrCreate(pushToken).Error
}

func (r *userRepository) DeletePushToken(userID uint, token string) error {
	db := r.GetDB()
	return db.Where("user_id = ? AND token = ?", userID, token).Delete(&models.PushToken{}).Error
}
