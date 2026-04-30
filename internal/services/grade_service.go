package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/utils"
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
	repo  repositories.GradeRepository
	cache CacheService
}

func NewGradeService(repo repositories.GradeRepository, cache CacheService) GradeService {
	return &gradeService{repo: repo, cache: cache}
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
	if s.cache != nil {
		_ = s.cache.DeletePattern("filter:*")
		_ = s.cache.DeletePattern("school-classes:*")
		_ = s.cache.DeletePattern("subjects:*")
		_ = s.cache.DeletePattern("semesters:*")
		_ = s.cache.DeletePattern("home:*")
	}

	cc := database.CountryCode(countryID)
	InvalidateCache(classesKey(cc), filterKey(cc))
}

// ── School Classes ──────────────────────────────────────────────────────────

func (s *gradeService) ListSchoolClasses(countryID database.CountryID) ([]models.SchoolClass, error) {
	cacheKey := utils.CacheKey("school-classes:list", countryID)

	var cached []models.SchoolClass
	if s.cache != nil && s.cache.Get(cacheKey, &cached) {
		return cached, nil
	}

	classes, err := s.repo.ListSchoolClasses(countryID)
	if err != nil {
		return nil, MapError(err)
	}

	if s.cache != nil {
		_ = s.cache.Set(cacheKey, classes, time.Hour)
	}

	return classes, nil
}

func (s *gradeService) GetSchoolClass(countryID database.CountryID, id uint64) (*models.SchoolClass, error) {
	class, err := s.repo.FindSchoolClassByID(countryID, id)
	return class, MapError(err)
}

func (s *gradeService) CreateSchoolClass(countryID database.CountryID, req *SchoolClassInput) (*models.SchoolClass, error) {
	class := &models.SchoolClass{
		GradeName:  req.GradeName,
		GradeLevel: req.GradeLevel,
	}

	if err := s.repo.CreateSchoolClass(countryID, class); err != nil {
		return nil, MapError(err)
	}
	s.InvalidateClassCache(countryID)
	return class, nil
}

func (s *gradeService) UpdateSchoolClass(countryID database.CountryID, id uint64, req *SchoolClassInput) (*models.SchoolClass, error) {
	class, err := s.repo.FindSchoolClassByID(countryID, id)
	if err != nil {
		return nil, MapError(err)
	}

	if req.GradeName != "" {
		class.GradeName = req.GradeName
	}
	if req.GradeLevel > 0 {
		class.GradeLevel = req.GradeLevel
	}

	if err := s.repo.UpdateSchoolClass(countryID, class); err != nil {
		return nil, MapError(err)
	}

	s.InvalidateClassCache(countryID)
	return class, nil
}

func (s *gradeService) DeleteSchoolClass(countryID database.CountryID, id uint64) error {
	if err := s.repo.DeleteSchoolClass(countryID, id); err != nil {
		return MapError(err)
	}
	s.InvalidateClassCache(countryID)
	return nil
}

func (s *gradeService) ListSchoolClassesDashboard(countryID database.CountryID, limit, offset int) ([]models.SchoolClass, int64, error) {
	total, err := s.repo.CountSchoolClasses(countryID)
	if err != nil {
		return nil, 0, MapError(err)
	}

	classes, err := s.repo.ListSchoolClassesPaginated(countryID, limit, offset)
	return classes, total, MapError(err)
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
		return nil, MapError(err)
	}

	if s.cache != nil {
		_ = s.cache.DeletePattern("filter:*")
		_ = s.cache.DeletePattern("subjects:*")
		_ = s.cache.DeletePattern("home:*")
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
	subject, err := s.repo.FindSubjectByID(countryID, id)
	return subject, MapError(err)
}

func (s *gradeService) UpdateSubject(countryID database.CountryID, id uint64, req *SubjectInput) (*models.Subject, error) {
	subject, err := s.repo.FindSubjectByID(countryID, id)
	if err != nil {
		return nil, MapError(err)
	}

	subject.SubjectName = req.SubjectName
	subject.GradeLevel = req.GradeLevel

	if err := s.repo.UpdateSubject(countryID, subject); err != nil {
		return nil, MapError(err)
	}

	if s.cache != nil {
		_ = s.cache.DeletePattern("filter:*")
		_ = s.cache.DeletePattern("subjects:*")
		_ = s.cache.DeletePattern("home:*")
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
		return MapError(err)
	}

	if err := s.repo.DeleteSubject(countryID, id); err != nil {
		return MapError(err)
	}

	if s.cache != nil {
		_ = s.cache.DeletePattern("filter:*")
		_ = s.cache.DeletePattern("subjects:*")
		_ = s.cache.DeletePattern("home:*")
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
		return nil, 0, MapError(err)
	}

	subjects, err := s.repo.ListSubjectsPaginated(countryID, limit, offset)
	return subjects, total, MapError(err)
}

// ── Semesters ───────────────────────────────────────────────────────────────

func (s *gradeService) ListSemesters(countryID database.CountryID, subjectID uint64) ([]models.Semester, *models.Subject, error) {
	subject, err := s.repo.FindSubjectByID(countryID, subjectID)
	if err != nil {
		return nil, nil, MapError(err)
	}

	key := database.Redis().CountryKey(database.CountryCode(countryID), "semesters", strconv.FormatUint(uint64(subject.GradeLevel), 10))
	semesters, err := GetOrSet[[]models.Semester](context.Background(), key, classesAndFilterTTL, func() ([]models.Semester, error) {
		return s.repo.ListSemestersByGradeLevel(countryID, subject.GradeLevel)
	})

	return semesters, subject, MapError(err)
}

func (s *gradeService) GetSemester(countryID database.CountryID, id uint64) (*models.Semester, error) {
	semester, err := s.repo.FindSemesterByID(countryID, id)
	return semester, MapError(err)
}

func (s *gradeService) CreateSemester(countryID database.CountryID, req *SemesterInput) (*models.Semester, error) {
	semester := &models.Semester{
		SemesterName: req.SemesterName,
		GradeLevel:   req.GradeLevel,
	}

	if err := s.repo.CreateSemester(countryID, semester); err != nil {
		return nil, MapError(err)
	}

	if s.cache != nil {
		_ = s.cache.DeletePattern("filter:*")
		_ = s.cache.DeletePattern("semesters:*")
		_ = s.cache.DeletePattern("home:*")
	}

	InvalidateCache(
		database.Redis().CountryKey(database.CountryCode(countryID), "semesters", strconv.FormatUint(uint64(semester.GradeLevel), 10)),
	)
	return semester, nil
}

func (s *gradeService) UpdateSemester(countryID database.CountryID, id uint64, req *SemesterInput) (*models.Semester, error) {
	semester, err := s.repo.FindSemesterByID(countryID, id)
	if err != nil {
		return nil, MapError(err)
	}

	if req.SemesterName != "" {
		semester.SemesterName = req.SemesterName
	}
	if req.GradeLevel > 0 {
		semester.GradeLevel = req.GradeLevel
	}

	if err := s.repo.UpdateSemester(countryID, semester); err != nil {
		return nil, MapError(err)
	}

	if s.cache != nil {
		_ = s.cache.DeletePattern("filter:*")
		_ = s.cache.DeletePattern("semesters:*")
		_ = s.cache.DeletePattern("home:*")
	}

	InvalidateCache(
		database.Redis().CountryKey(database.CountryCode(countryID), "semesters", strconv.FormatUint(uint64(semester.GradeLevel), 10)),
	)
	return semester, nil
}

func (s *gradeService) DeleteSemester(countryID database.CountryID, id uint64) error {
	semester, err := s.repo.FindSemesterByID(countryID, id)
	if err != nil {
		return MapError(err)
	}

	if err := s.repo.DeleteSemester(countryID, id); err != nil {
		return MapError(err)
	}

	if s.cache != nil {
		_ = s.cache.DeletePattern("filter:*")
		_ = s.cache.DeletePattern("semesters:*")
		_ = s.cache.DeletePattern("home:*")
	}

	InvalidateCache(
		database.Redis().CountryKey(database.CountryCode(countryID), "semesters", strconv.FormatUint(uint64(semester.GradeLevel), 10)),
	)
	return nil
}

func (s *gradeService) ListSemestersDashboard(countryID database.CountryID, limit, offset int) ([]models.Semester, int64, error) {
	total, err := s.repo.CountSemesters(countryID)
	if err != nil {
		return nil, 0, MapError(err)
	}

	semesters, err := s.repo.ListSemestersPaginated(countryID, limit, offset)
	return semesters, total, MapError(err)
}

// ── Meta / Filter ───────────────────────────────────────────────────────────

type filterResult struct {
	Classes []models.SchoolClass `json:"classes"`
}

func (s *gradeService) FilterMeta(countryID database.CountryID) ([]models.SchoolClass, error) {
	cacheKey := utils.CacheKey("filter:meta", countryID)

	var cached filterResult
	if s.cache != nil && s.cache.Get(cacheKey, &cached) {
		return cached.Classes, nil
	}

	classes, err := s.repo.ListSchoolClasses(countryID)
	if err != nil {
		return nil, MapError(err)
	}

	if s.cache != nil {
		_ = s.cache.Set(cacheKey, filterResult{Classes: classes}, time.Hour)
	}

	return classes, nil
}

// ── Grade Articles ──────────────────────────────────────────────────────────

func (s *gradeService) ListGradeArticles(countryID database.CountryID, subjectID uint64, limit, offset int) ([]models.Article, int64, error) {
	key := database.Redis().CountryKey(database.CountryCode(countryID), "grade-articles", fmt.Sprintf("sub%d-l%d-o%d", subjectID, limit, offset))

	type listResult struct {
		Articles []models.Article `json:"articles"`
		Total    int64            `json:"total"`
	}

	res, err := GetOrSet[listResult](context.Background(), key, 5*time.Minute, func() (listResult, error) {
		total, err := s.repo.CountGradeArticles(countryID, subjectID)
		if err != nil {
			return listResult{}, err
		}

		articles, err := s.repo.ListGradeArticlesPaginated(countryID, subjectID, limit, offset)
		return listResult{Articles: articles, Total: total}, err
	})

	return res.Articles, res.Total, MapError(err)
}
