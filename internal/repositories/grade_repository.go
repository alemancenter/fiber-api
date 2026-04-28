package repositories

import (
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"gorm.io/gorm"
)

type GradeRepository interface {
	GetDB(countryID database.CountryID) *gorm.DB

	// School Classes
	ListSchoolClasses(countryID database.CountryID) ([]models.SchoolClass, error)
	FindSchoolClassByID(countryID database.CountryID, id uint64) (*models.SchoolClass, error)
	CreateSchoolClass(countryID database.CountryID, class *models.SchoolClass) error
	UpdateSchoolClass(countryID database.CountryID, class *models.SchoolClass) error
	DeleteSchoolClass(countryID database.CountryID, id uint64) error
	CountSchoolClasses(countryID database.CountryID) (int64, error)
	ListSchoolClassesPaginated(countryID database.CountryID, limit, offset int) ([]models.SchoolClass, error)

	// Subjects
	ListSubjectsByClassID(countryID database.CountryID, classID uint64) ([]models.Subject, error)
	FindSubjectByID(countryID database.CountryID, id uint64) (*models.Subject, error)
	CreateSubject(countryID database.CountryID, subject *models.Subject) error
	CountSubjects(countryID database.CountryID) (int64, error)
	ListSubjectsPaginated(countryID database.CountryID, limit, offset int) ([]models.Subject, error)

	// Semesters
	ListSemestersByGradeLevel(countryID database.CountryID, gradeLevel uint) ([]models.Semester, error)
	FindSemesterByID(countryID database.CountryID, id uint64) (*models.Semester, error)
	CreateSemester(countryID database.CountryID, semester *models.Semester) error
	UpdateSemester(countryID database.CountryID, semester *models.Semester) error
	DeleteSemester(countryID database.CountryID, id uint64) error
	CountSemesters(countryID database.CountryID) (int64, error)
	ListSemestersPaginated(countryID database.CountryID, limit, offset int) ([]models.Semester, error)

	// Articles
	CountGradeArticles(countryID database.CountryID, subjectID uint64) (int64, error)
	ListGradeArticlesPaginated(countryID database.CountryID, subjectID uint64, limit, offset int) ([]models.Article, error)
}

type gradeRepository struct{}

func NewGradeRepository() GradeRepository {
	return &gradeRepository{}
}

func (r *gradeRepository) GetDB(countryID database.CountryID) *gorm.DB {
	return database.DBForCountry(countryID)
}

// ── School Classes ──────────────────────────────────────────────────────────

func (r *gradeRepository) ListSchoolClasses(countryID database.CountryID) ([]models.SchoolClass, error) {
	var list []models.SchoolClass
	err := r.GetDB(countryID).Order("grade_level ASC, grade_name ASC").Find(&list).Error
	return list, err
}

func (r *gradeRepository) FindSchoolClassByID(countryID database.CountryID, id uint64) (*models.SchoolClass, error) {
	var class models.SchoolClass
	err := r.GetDB(countryID).Preload("Subjects").First(&class, id).Error
	return &class, err
}

func (r *gradeRepository) CreateSchoolClass(countryID database.CountryID, class *models.SchoolClass) error {
	return r.GetDB(countryID).Create(class).Error
}

func (r *gradeRepository) UpdateSchoolClass(countryID database.CountryID, class *models.SchoolClass) error {
	return r.GetDB(countryID).Save(class).Error
}

func (r *gradeRepository) DeleteSchoolClass(countryID database.CountryID, id uint64) error {
	return r.GetDB(countryID).Delete(&models.SchoolClass{}, id).Error
}

func (r *gradeRepository) CountSchoolClasses(countryID database.CountryID) (int64, error) {
	var total int64
	err := r.GetDB(countryID).Model(&models.SchoolClass{}).Count(&total).Error
	return total, err
}

func (r *gradeRepository) ListSchoolClassesPaginated(countryID database.CountryID, limit, offset int) ([]models.SchoolClass, error) {
	var classes []models.SchoolClass
	err := r.GetDB(countryID).Order("grade_level ASC, grade_name ASC").Limit(limit).Offset(offset).Find(&classes).Error
	return classes, err
}

// ── Subjects ────────────────────────────────────────────────────────────────

func (r *gradeRepository) ListSubjectsByClassID(countryID database.CountryID, classID uint64) ([]models.Subject, error) {
	var list []models.Subject
	err := r.GetDB(countryID).Where("grade_level = ?", classID).Order("subject_name ASC").Find(&list).Error
	return list, err
}

func (r *gradeRepository) FindSubjectByID(countryID database.CountryID, id uint64) (*models.Subject, error) {
	var subject models.Subject
	err := r.GetDB(countryID).First(&subject, id).Error
	return &subject, err
}

func (r *gradeRepository) CreateSubject(countryID database.CountryID, subject *models.Subject) error {
	return r.GetDB(countryID).Create(subject).Error
}

func (r *gradeRepository) CountSubjects(countryID database.CountryID) (int64, error) {
	var total int64
	err := r.GetDB(countryID).Model(&models.Subject{}).Count(&total).Error
	return total, err
}

func (r *gradeRepository) ListSubjectsPaginated(countryID database.CountryID, limit, offset int) ([]models.Subject, error) {
	var subjects []models.Subject
	err := r.GetDB(countryID).Preload("SchoolClass").Order("subject_name ASC").Limit(limit).Offset(offset).Find(&subjects).Error
	return subjects, err
}

// ── Semesters ───────────────────────────────────────────────────────────────

func (r *gradeRepository) ListSemestersByGradeLevel(countryID database.CountryID, gradeLevel uint) ([]models.Semester, error) {
	var list []models.Semester
	err := r.GetDB(countryID).Where("grade_level = ?", gradeLevel).Order("semester_name ASC").Find(&list).Error
	return list, err
}

func (r *gradeRepository) FindSemesterByID(countryID database.CountryID, id uint64) (*models.Semester, error) {
	var semester models.Semester
	err := r.GetDB(countryID).First(&semester, id).Error
	return &semester, err
}

func (r *gradeRepository) CreateSemester(countryID database.CountryID, semester *models.Semester) error {
	return r.GetDB(countryID).Create(semester).Error
}

func (r *gradeRepository) UpdateSemester(countryID database.CountryID, semester *models.Semester) error {
	return r.GetDB(countryID).Save(semester).Error
}

func (r *gradeRepository) DeleteSemester(countryID database.CountryID, id uint64) error {
	return r.GetDB(countryID).Delete(&models.Semester{}, id).Error
}

func (r *gradeRepository) CountSemesters(countryID database.CountryID) (int64, error) {
	var total int64
	err := r.GetDB(countryID).Model(&models.Semester{}).Count(&total).Error
	return total, err
}

func (r *gradeRepository) ListSemestersPaginated(countryID database.CountryID, limit, offset int) ([]models.Semester, error) {
	var semesters []models.Semester
	err := r.GetDB(countryID).Preload("SchoolClass").Order("semester_name ASC").Limit(limit).Offset(offset).Find(&semesters).Error
	return semesters, err
}

// ── Articles ────────────────────────────────────────────────────────────────

func (r *gradeRepository) CountGradeArticles(countryID database.CountryID, subjectID uint64) (int64, error) {
	var total int64
	err := r.GetDB(countryID).Model(&models.Article{}).Where("subject_id = ? AND status = ?", subjectID, 1).Count(&total).Error
	return total, err
}

func (r *gradeRepository) ListGradeArticlesPaginated(countryID database.CountryID, subjectID uint64, limit, offset int) ([]models.Article, error) {
	var articles []models.Article
	err := r.GetDB(countryID).Preload("Subject").Preload("Semester").Preload("Files").
		Where("subject_id = ? AND status = ?", subjectID, 1).
		Order("published_at DESC").
		Limit(limit).Offset(offset).
		Find(&articles).Error
	return articles, err
}
