package repositories

import (
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/utils"
	"gorm.io/gorm"
)

type ArticleRepository interface {
	GetDB(countryID database.CountryID) *gorm.DB
	List(countryID database.CountryID, pag utils.Pagination, filters map[string]interface{}) ([]models.Article, int64, error)
	FindByID(countryID database.CountryID, id uint64) (*models.Article, error)
	FindByGradeLevel(countryID database.CountryID, gradeLevel string, pag utils.Pagination) ([]models.Article, int64, error)
	FindByKeyword(countryID database.CountryID, keyword string, pag utils.Pagination) ([]models.Article, int64, error)
	Create(countryID database.CountryID, article *models.Article) error
	Update(countryID database.CountryID, article *models.Article, updates map[string]interface{}) error
	Delete(countryID database.CountryID, article *models.Article) error
	GetFileByID(countryID database.CountryID, id uint64) (*models.File, error)
	IncrementViewCount(countryID database.CountryID, articleID uint64) error
	IncrementFileViewCount(countryID database.CountryID, fileID uint64) error
	GetClasses(countryID database.CountryID) ([]models.SchoolClass, error)
	GetSubjectsByClass(countryID database.CountryID, classID uint) ([]models.Subject, error)
	GetSubjectByID(countryID database.CountryID, subjectID uint) (*models.Subject, error)
	GetSemestersByClass(countryID database.CountryID, classID uint) ([]models.Semester, error)
	GetStats(countryID database.CountryID) (total, published, drafts, views int64, err error)
}

type articleRepository struct{}

func NewArticleRepository() ArticleRepository {
	return &articleRepository{}
}

func (r *articleRepository) GetDB(countryID database.CountryID) *gorm.DB {
	return database.DBForCountry(countryID)
}

func (r *articleRepository) List(countryID database.CountryID, pag utils.Pagination, filters map[string]interface{}) ([]models.Article, int64, error) {
	db := r.GetDB(countryID)
	var articles []models.Article
	var total int64

	query := db.Model(&models.Article{}).Preload("Subject").Preload("Semester").Preload("Files")

	if status, ok := filters["status"]; ok {
		query = query.Where("status = ?", status)
	} else {
		// By default only published in List?
		// Actually, we pass status=1 from List and no status or specific status from DashboardList.
	}

	if gradeLevel, ok := filters["grade_level"]; ok && gradeLevel != "" {
		query = query.Where("grade_level = ?", gradeLevel)
	}
	if subjectID, ok := filters["subject_id"]; ok && subjectID != "" {
		query = query.Where("subject_id = ?", subjectID)
	}
	if semesterID, ok := filters["semester_id"]; ok && semesterID != "" {
		query = query.Where("semester_id = ?", semesterID)
	}
	if q, ok := filters["q"]; ok && q != "" {
		query = query.Where("title LIKE ?", "%"+q.(string)+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Handle custom order if provided, else default to published_at DESC, created_at DESC
	if order, ok := filters["order"]; ok {
		query = query.Order(order)
	} else {
		query = query.Order("published_at DESC, created_at DESC")
	}

	err := query.Limit(pag.PerPage).Offset(pag.Offset).Find(&articles).Error
	return articles, total, err
}

func (r *articleRepository) FindByID(countryID database.CountryID, id uint64) (*models.Article, error) {
	db := r.GetDB(countryID)
	var article models.Article
	err := db.Preload("Subject").Preload("Semester").Preload("Files").
		Preload("Comments.User").Preload("KeywordsRel").
		First(&article, id).Error
	return &article, err
}

func (r *articleRepository) FindByGradeLevel(countryID database.CountryID, gradeLevel string, pag utils.Pagination) ([]models.Article, int64, error) {
	db := r.GetDB(countryID)
	var articles []models.Article
	var total int64

	query := db.Model(&models.Article{}).Where("grade_level = ? AND status = ?", gradeLevel, 1)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("Subject").Preload("Semester").
		Order("published_at DESC").
		Limit(pag.PerPage).Offset(pag.Offset).
		Find(&articles).Error
	return articles, total, err
}

func (r *articleRepository) FindByKeyword(countryID database.CountryID, keyword string, pag utils.Pagination) ([]models.Article, int64, error) {
	db := r.GetDB(countryID)
	var articles []models.Article
	var total int64

	var kw models.Keyword
	if err := db.Where("keyword = ?", keyword).First(&kw).Error; err != nil {
		return nil, 0, err // Not found or error
	}

	subQuery := db.Table("article_keyword").Select("article_id").Where("keyword_id = ?", kw.ID)
	query := db.Model(&models.Article{}).Where("id IN (?) AND status = ?", subQuery, 1)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("Subject").Preload("Semester").
		Order("published_at DESC").
		Limit(pag.PerPage).Offset(pag.Offset).
		Find(&articles).Error

	return articles, total, err
}

func (r *articleRepository) Create(countryID database.CountryID, article *models.Article) error {
	return r.GetDB(countryID).Create(article).Error
}

func (r *articleRepository) Update(countryID database.CountryID, article *models.Article, updates map[string]interface{}) error {
	return r.GetDB(countryID).Model(article).Updates(updates).Error
}

func (r *articleRepository) Delete(countryID database.CountryID, article *models.Article) error {
	return r.GetDB(countryID).Delete(article).Error
}

func (r *articleRepository) GetFileByID(countryID database.CountryID, id uint64) (*models.File, error) {
	db := r.GetDB(countryID)
	var file models.File
	err := db.First(&file, id).Error
	return &file, err
}

func (r *articleRepository) IncrementViewCount(countryID database.CountryID, articleID uint64) error {
	db := r.GetDB(countryID)
	return db.Model(&models.Article{}).Where("id = ?", articleID).UpdateColumn("visit_count", gorm.Expr("visit_count + 1")).Error
}

func (r *articleRepository) IncrementFileViewCount(countryID database.CountryID, fileID uint64) error {
	db := r.GetDB(countryID)
	return db.Model(&models.File{}).Where("id = ?", fileID).UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

func (r *articleRepository) GetClasses(countryID database.CountryID) ([]models.SchoolClass, error) {
	var classes []models.SchoolClass
	err := r.GetDB(countryID).Order("grade_level ASC, grade_name ASC").Find(&classes).Error
	return classes, err
}

func (r *articleRepository) GetSubjectsByClass(countryID database.CountryID, classID uint) ([]models.Subject, error) {
	var subjects []models.Subject
	err := r.GetDB(countryID).Where("grade_level = ?", classID).Order("subject_name ASC").Find(&subjects).Error
	return subjects, err
}

func (r *articleRepository) GetSubjectByID(countryID database.CountryID, subjectID uint) (*models.Subject, error) {
	var subject models.Subject
	err := r.GetDB(countryID).First(&subject, subjectID).Error
	return &subject, err
}

func (r *articleRepository) GetSemestersByClass(countryID database.CountryID, classID uint) ([]models.Semester, error) {
	var semesters []models.Semester
	err := r.GetDB(countryID).Where("grade_level = ?", classID).Order("semester_name ASC").Find(&semesters).Error
	return semesters, err
}

func (r *articleRepository) GetStats(countryID database.CountryID) (total, published, drafts, views int64, err error) {
	db := r.GetDB(countryID)
	db.Model(&models.Article{}).Count(&total)
	db.Model(&models.Article{}).Where("status = ?", 1).Count(&published)
	db.Model(&models.Article{}).Where("status = ?", 0).Count(&drafts)
	err = db.Model(&models.Article{}).Select("COALESCE(SUM(visit_count), 0)").Scan(&views).Error
	return
}
