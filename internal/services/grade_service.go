package services

import (
	"context"
	"strconv"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
)

const (
	classesAndFilterTTL = time.Hour
)

type GradeService interface {
	// School Classes
	ListSchoolClasses(countryID database.CountryID) ([]models.SchoolClass, error)
	GetSchoolClass(countryID database.CountryID, id uint64) (*models.SchoolClass, error)
	CreateSchoolClass(countryID database.CountryID, req *SchoolClassInput) (*models.SchoolClass, error)
	UpdateSchoolClass(countryID database.CountryID, id uint64, req *SchoolClassInput) (*models.SchoolClass, error)
	DeleteSchoolClass(countryID database.CountryID, id uint64) error
	ListSchoolClassesDashboard(countryID database.CountryID, limit, offset int) ([]models.SchoolClass, int64, error)
	InvalidateClassCache(countryID database.CountryID)

	// Subjects
	ListSubjects(countryID database.CountryID, classID uint64) ([]models.Subject, error)
	GetSubject(countryID database.CountryID, id uint64) (*models.Subject, error)
	CreateSubject(countryID database.CountryID, req *SubjectInput) (*models.Subject, error)
	UpdateSubject(countryID database.CountryID, id uint64, req *SubjectInput) (*models.Subject, error)
	DeleteSubject(countryID database.CountryID, id uint64) error
	ListSubjectsDashboard(countryID database.CountryID, limit, offset int) ([]models.Subject, int64, error)

	// Semesters
	ListSemesters(countryID database.CountryID, subjectID uint64) ([]models.Semester, *models.Subject, error)
	GetSemester(countryID database.CountryID, id uint64) (*models.Semester, error)
	CreateSemester(countryID database.CountryID, req *SemesterInput) (*models.Semester, error)
	UpdateSemester(countryID database.CountryID, id uint64, req *SemesterInput) (*models.Semester, error)
	DeleteSemester(countryID database.CountryID, id uint64) error
	ListSemestersDashboard(countryID database.CountryID, limit, offset int) ([]models.Semester, int64, error)

	// Meta / Filter
	FilterMeta(countryID database.CountryID) ([]models.SchoolClass, error)

	// Grade Articles
	ListGradeArticles(countryID database.CountryID, subjectID uint64, limit, offset int) ([]models.Article, int64, error)
}

type SchoolClassInput struct {
	GradeName  string `json:"grade_name" validate:"required,min=2,max=255"`
	GradeLevel int    `json:"grade_level"`
}

type SubjectInput struct {
	SubjectName string `json:"subject_name" validate:"required,min=2,max=255"`
	GradeLevel  uint   `json:"grade_level" validate:"required"`
}

type SemesterInput struct {
	SemesterName string `json:"semester_name" validate:"required,min=2,max=255"`
	GradeLevel   uint   `json:"grade_level"`
}

type SemestersResponse struct {
	Subject   *models.Subject   `json:"subject"`
	ClassID   uint              `json:"class_id"`
	Semesters []models.Semester `json:"semesters"`
}

type FilterMetaResponse struct {
	Classes []models.SchoolClass `json:"classes"`
}

type gradeService struct {
	repo repositories.GradeRepository
}

func NewGradeService(repo repositories.GradeRepository) GradeService {
	return &gradeService{repo: repo}
}

// ── Cache Keys ──────────────────────────────────────────────────────────────

func classesKey(country string) string {
	return database.Redis().CountryKey(country, "school-classes")
}

func filterKey(country string) string {
	return database.Redis().CountryKey(country, "filter-meta")
}

// InvalidateClassCache removes school class and filter caches for a country.
func (s *gradeService) InvalidateClassCache(countryID database.CountryID) {
	cc := database.CountryCode(countryID)
	InvalidateCache(classesKey(cc), filterKey(cc))
}

// ── School Classes ──────────────────────────────────────────────────────────

func (s *gradeService) ListSchoolClasses(countryID database.CountryID) ([]models.SchoolClass, error) {
	key := classesKey(database.CountryCode(countryID))

	return GetOrSet[[]models.SchoolClass](context.Background(), key, classesAndFilterTTL, func() ([]models.SchoolClass, error) {
		return s.repo.ListSchoolClasses(countryID)
	})
}

func (s *gradeService) GetSchoolClass(countryID database.CountryID, id uint64) (*models.SchoolClass, error) {
	return s.repo.FindSchoolClassByID(countryID, id)
}

func (s *gradeService) CreateSchoolClass(countryID database.CountryID, req *SchoolClassInput) (*models.SchoolClass, error) {
	class := &models.SchoolClass{
		GradeName:  req.GradeName,
		GradeLevel: req.GradeLevel,
	}

	if err := s.repo.CreateSchoolClass(countryID, class); err != nil {
		return nil, err
	}
	s.InvalidateClassCache(countryID)
	return class, nil
}

func (s *gradeService) UpdateSchoolClass(countryID database.CountryID, id uint64, req *SchoolClassInput) (*models.SchoolClass, error) {
	class, err := s.repo.FindSchoolClassByID(countryID, id)
	if err != nil {
		return nil, err
	}

	if req.GradeName != "" {
		class.GradeName = req.GradeName
	}
	if req.GradeLevel > 0 {
		class.GradeLevel = req.GradeLevel
	}

	if err := s.repo.UpdateSchoolClass(countryID, class); err != nil {
		return nil, err
	}

	s.InvalidateClassCache(countryID)
	return class, nil
}

func (s *gradeService) DeleteSchoolClass(countryID database.CountryID, id uint64) error {
	if err := s.repo.DeleteSchoolClass(countryID, id); err != nil {
		return err
	}
	s.InvalidateClassCache(countryID)
	return nil
}

func (s *gradeService) ListSchoolClassesDashboard(countryID database.CountryID, limit, offset int) ([]models.SchoolClass, int64, error) {
	total, err := s.repo.CountSchoolClasses(countryID)
	if err != nil {
		return nil, 0, err
	}

	classes, err := s.repo.ListSchoolClassesPaginated(countryID, limit, offset)
	return classes, total, err
}

// ── Subjects ────────────────────────────────────────────────────────────────

func (s *gradeService) ListSubjects(countryID database.CountryID, classID uint64) ([]models.Subject, error) {
	key := database.Redis().CountryKey(database.CountryCode(countryID), "subjects", strconv.FormatUint(classID, 10))

	return GetOrSet[[]models.Subject](context.Background(), key, classesAndFilterTTL, func() ([]models.Subject, error) {
		return s.repo.ListSubjectsByClassID(countryID, classID)
	})
}

func (s *gradeService) CreateSubject(countryID database.CountryID, req *SubjectInput) (*models.Subject, error) {
	subject := &models.Subject{
		SubjectName: req.SubjectName,
		GradeLevel:  req.GradeLevel,
	}

	if err := s.repo.CreateSubject(countryID, subject); err != nil {
		return nil, err
	}

	// Invalidate subjects cache for this class
	cc := database.CountryCode(countryID)
	InvalidateCache(
		database.Redis().CountryKey(cc, "subjects", strconv.FormatUint(uint64(subject.GradeLevel), 10)),
		filterKey(cc),
	)
	return subject, nil
}

func (s *gradeService) GetSubject(countryID database.CountryID, id uint64) (*models.Subject, error) {
	return s.repo.FindSubjectByID(countryID, id)
}

func (s *gradeService) UpdateSubject(countryID database.CountryID, id uint64, req *SubjectInput) (*models.Subject, error) {
	subject, err := s.repo.FindSubjectByID(countryID, id)
	if err != nil {
		return nil, err
	}

	subject.SubjectName = req.SubjectName
	subject.GradeLevel = req.GradeLevel

	if err := s.repo.UpdateSubject(countryID, subject); err != nil {
		return nil, err
	}

	cc := database.CountryCode(countryID)
	InvalidateCache(
		database.Redis().CountryKey(cc, "subjects", strconv.FormatUint(uint64(subject.GradeLevel), 10)),
		filterKey(cc),
	)
	return subject, nil
}

func (s *gradeService) DeleteSubject(countryID database.CountryID, id uint64) error {
	subject, err := s.repo.FindSubjectByID(countryID, id)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteSubject(countryID, id); err != nil {
		return err
	}

	cc := database.CountryCode(countryID)
	InvalidateCache(
		database.Redis().CountryKey(cc, "subjects", strconv.FormatUint(uint64(subject.GradeLevel), 10)),
		filterKey(cc),
	)
	return nil
}

func (s *gradeService) ListSubjectsDashboard(countryID database.CountryID, limit, offset int) ([]models.Subject, int64, error) {
	total, err := s.repo.CountSubjects(countryID)
	if err != nil {
		return nil, 0, err
	}

	subjects, err := s.repo.ListSubjectsPaginated(countryID, limit, offset)
	return subjects, total, err
}

// ── Semesters ───────────────────────────────────────────────────────────────

func (s *gradeService) ListSemesters(countryID database.CountryID, subjectID uint64) ([]models.Semester, *models.Subject, error) {
	subject, err := s.repo.FindSubjectByID(countryID, subjectID)
	if err != nil {
		return nil, nil, err
	}

	key := database.Redis().CountryKey(database.CountryCode(countryID), "semesters", strconv.FormatUint(uint64(subject.GradeLevel), 10))
	semesters, err := GetOrSet[[]models.Semester](context.Background(), key, classesAndFilterTTL, func() ([]models.Semester, error) {
		return s.repo.ListSemestersByGradeLevel(countryID, subject.GradeLevel)
	})

	return semesters, subject, err
}

func (s *gradeService) GetSemester(countryID database.CountryID, id uint64) (*models.Semester, error) {
	return s.repo.FindSemesterByID(countryID, id)
}

func (s *gradeService) CreateSemester(countryID database.CountryID, req *SemesterInput) (*models.Semester, error) {
	semester := &models.Semester{
		SemesterName: req.SemesterName,
		GradeLevel:   req.GradeLevel,
	}

	if err := s.repo.CreateSemester(countryID, semester); err != nil {
		return nil, err
	}

	InvalidateCache(
		database.Redis().CountryKey(database.CountryCode(countryID), "semesters", strconv.FormatUint(uint64(semester.GradeLevel), 10)),
	)
	return semester, nil
}

func (s *gradeService) UpdateSemester(countryID database.CountryID, id uint64, req *SemesterInput) (*models.Semester, error) {
	semester, err := s.repo.FindSemesterByID(countryID, id)
	if err != nil {
		return nil, err
	}

	if req.SemesterName != "" {
		semester.SemesterName = req.SemesterName
	}
	if req.GradeLevel > 0 {
		semester.GradeLevel = req.GradeLevel
	}

	if err := s.repo.UpdateSemester(countryID, semester); err != nil {
		return nil, err
	}

	InvalidateCache(
		database.Redis().CountryKey(database.CountryCode(countryID), "semesters", strconv.FormatUint(uint64(semester.GradeLevel), 10)),
	)
	return semester, nil
}

func (s *gradeService) DeleteSemester(countryID database.CountryID, id uint64) error {
	semester, err := s.repo.FindSemesterByID(countryID, id)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteSemester(countryID, id); err != nil {
		return err
	}

	InvalidateCache(
		database.Redis().CountryKey(database.CountryCode(countryID), "semesters", strconv.FormatUint(uint64(semester.GradeLevel), 10)),
	)
	return nil
}

func (s *gradeService) ListSemestersDashboard(countryID database.CountryID, limit, offset int) ([]models.Semester, int64, error) {
	total, err := s.repo.CountSemesters(countryID)
	if err != nil {
		return nil, 0, err
	}

	semesters, err := s.repo.ListSemestersPaginated(countryID, limit, offset)
	return semesters, total, err
}

// ── Meta / Filter ───────────────────────────────────────────────────────────

type filterResult struct {
	Classes []models.SchoolClass `json:"classes"`
}

func (s *gradeService) FilterMeta(countryID database.CountryID) ([]models.SchoolClass, error) {
	key := filterKey(database.CountryCode(countryID))

	result, err := GetOrSet[filterResult](context.Background(), key, classesAndFilterTTL, func() (filterResult, error) {
		classes, err := s.repo.ListSchoolClasses(countryID)
		return filterResult{Classes: classes}, err
	})

	return result.Classes, err
}

// ── Grade Articles ──────────────────────────────────────────────────────────

func (s *gradeService) ListGradeArticles(countryID database.CountryID, subjectID uint64, limit, offset int) ([]models.Article, int64, error) {
	total, err := s.repo.CountGradeArticles(countryID, subjectID)
	if err != nil {
		return nil, 0, err
	}

	articles, err := s.repo.ListGradeArticlesPaginated(countryID, subjectID, limit, offset)
	return articles, total, err
}
