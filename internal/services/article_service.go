package services

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

type ArticleInput struct {
	Title           string `json:"title" validate:"required,min=3,max=500"`
	Content         string `json:"content" validate:"required"`
	GradeLevel      string `json:"grade_level"`
	SubjectID       *uint  `json:"subject_id"`
	SemesterID      *uint  `json:"semester_id"`
	MetaDescription string `json:"meta_description" validate:"omitempty,max=500"`
	Keywords        string `json:"keywords"`
	Status          *int8  `json:"status" validate:"omitempty,oneof=0 1"`
}

type ArticleService interface {
	List(countryID database.CountryID, pag utils.Pagination, filter *models.ArticleFilter) ([]models.Article, int64, error)
	GetByID(countryID database.CountryID, id uint64) (*models.Article, error)
	GetByGradeLevel(countryID database.CountryID, gradeLevel string, pag utils.Pagination) ([]models.Article, int64, error)
	GetByKeyword(countryID database.CountryID, keyword string, pag utils.Pagination) ([]models.Article, int64, error)
	GetFileForDownload(countryID database.CountryID, id uint64) (*models.File, string, error)

	// Dashboard methods
	GetDashboardCreateData(countryID database.CountryID) (fiber.Map, error)
	GetDashboardEditData(countryID database.CountryID, id uint64) (fiber.Map, error)
	CreateArticle(countryID database.CountryID, req *ArticleInput, authorID *uint) (*models.Article, error)
	UpdateArticle(countryID database.CountryID, id uint64, req *ArticleInput, authorID *uint) (*models.Article, error)
	DeleteArticle(countryID database.CountryID, id uint64, authorID *uint) error
	SetArticleStatus(countryID database.CountryID, id uint64, status int8) (*models.Article, error)
	GetDashboardStats(countryID database.CountryID) (fiber.Map, error)
}

type articleService struct {
	repo    repositories.ArticleRepository
	fileSvc *FileService
}

func NewArticleService(repo repositories.ArticleRepository, fileSvc *FileService) ArticleService {
	return &articleService{
		repo:    repo,
		fileSvc: fileSvc,
	}
}

func (s *articleService) List(countryID database.CountryID, pag utils.Pagination, filter *models.ArticleFilter) ([]models.Article, int64, error) {
	return s.repo.List(countryID, pag, filter)
}

func (s *articleService) GetByID(countryID database.CountryID, id uint64) (*models.Article, error) {
	article, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return nil, err
	}
	// Increment view count async
	go func() {
		_ = s.repo.IncrementViewCount(countryID, id)
	}()
	return article, nil
}

func (s *articleService) GetByGradeLevel(countryID database.CountryID, gradeLevel string, pag utils.Pagination) ([]models.Article, int64, error) {
	return s.repo.FindByGradeLevel(countryID, gradeLevel, pag)
}

func (s *articleService) GetByKeyword(countryID database.CountryID, keyword string, pag utils.Pagination) ([]models.Article, int64, error) {
	return s.repo.FindByKeyword(countryID, keyword, pag)
}

func (s *articleService) GetFileForDownload(countryID database.CountryID, id uint64) (*models.File, string, error) {
	file, err := s.repo.GetFileByID(countryID, id)
	if err != nil {
		return nil, "", err
	}

	absPath := s.fileSvc.GetAbsPath(file.FilePath)

	// Increment view count async
	go func() {
		_ = s.repo.IncrementFileViewCount(countryID, id)
	}()

	return file, absPath, nil
}

func (s *articleService) GetDashboardCreateData(countryID database.CountryID) (fiber.Map, error) {
	classes, err := s.repo.GetClasses(countryID)
	if err != nil {
		return nil, err
	}
	return fiber.Map{
		"classes":   classes,
		"subjects":  []models.Subject{},
		"semesters": []models.Semester{},
	}, nil
}

func (s *articleService) GetDashboardEditData(countryID database.CountryID, id uint64) (fiber.Map, error) {
	article, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return nil, err
	}

	classes, err := s.repo.GetClasses(countryID)
	if err != nil {
		return nil, err
	}

	var subjects []models.Subject
	var semesters []models.Semester

	classID := articleClassID(article)
	if classID > 0 {
		subjects, _ = s.repo.GetSubjectsByClass(countryID, classID)
	}

	if classID == 0 && article.SubjectID != nil {
		subject, err := s.repo.GetSubjectByID(countryID, *article.SubjectID)
		if err == nil {
			classID = subject.GradeLevel
		}
	}

	if classID > 0 {
		semesters, _ = s.repo.GetSemestersByClass(countryID, classID)
	}

	return fiber.Map{
		"data":      article,
		"classes":   classes,
		"subjects":  subjects,
		"semesters": semesters,
	}, nil
}

func (s *articleService) CreateArticle(countryID database.CountryID, req *ArticleInput, authorID *uint) (*models.Article, error) {
	article := &models.Article{
		Title:   utils.SanitizeInput(req.Title),
		Content: req.Content,
	}

	if req.Status != nil {
		article.Status = *req.Status
	}

	if req.GradeLevel != "" {
		article.GradeLevel = &req.GradeLevel
	}
	if req.SubjectID != nil && *req.SubjectID > 0 {
		article.SubjectID = req.SubjectID
	}
	if req.SemesterID != nil && *req.SemesterID > 0 {
		article.SemesterID = req.SemesterID
	}
	if req.MetaDescription != "" {
		article.MetaDescription = &req.MetaDescription
	}
	if req.Keywords != "" {
		article.Keywords = &req.Keywords
	}

	if authorID != nil {
		article.AuthorID = authorID
	}

	err := s.repo.Create(countryID, article)
	if err != nil {
		return nil, err
	}
	if authorID != nil {
		LogActivity("أنشأ مقالة: "+article.Title, "Article", article.ID, *authorID)
	}
	return article, nil
}

func (s *articleService) UpdateArticle(countryID database.CountryID, id uint64, req *ArticleInput, authorID *uint) (*models.Article, error) {
	article, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return nil, err
	}

	if req.Title != "" {
		article.Title = utils.SanitizeInput(req.Title)
	}
	if req.Content != "" {
		article.Content = req.Content
	}
	if req.GradeLevel != "" {
		article.GradeLevel = &req.GradeLevel
	}
	if req.SubjectID != nil {
		article.SubjectID = req.SubjectID
	}
	if req.SemesterID != nil {
		article.SemesterID = req.SemesterID
	}
	if req.MetaDescription != "" {
		article.MetaDescription = &req.MetaDescription
	}
	if req.Keywords != "" {
		article.Keywords = &req.Keywords
	}
	if req.Status != nil {
		article.Status = *req.Status
	}

	err = s.repo.Update(countryID, article)
	if err != nil {
		return nil, err
	}

	if authorID != nil {
		LogActivity("حدّث مقالة: "+article.Title, "Article", article.ID, *authorID)
	}

	return article, nil
}

func (s *articleService) DeleteArticle(countryID database.CountryID, id uint64, authorID *uint) error {
	article, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return err
	}

	err = s.repo.Delete(countryID, article)
	if err == nil && authorID != nil {
		LogActivity("حذف مقالة: "+article.Title, "Article", article.ID, *authorID)
	}

	return err
}

func (s *articleService) SetArticleStatus(countryID database.CountryID, id uint64, status int8) (*models.Article, error) {
	article, err := s.repo.FindByID(countryID, id)
	if err != nil {
		return nil, err
	}

	article.Status = status
	err = s.repo.Update(countryID, article)
	return article, err
}

func (s *articleService) GetDashboardStats(countryID database.CountryID) (fiber.Map, error) {
	total, published, drafts, views, err := s.repo.GetStats(countryID)
	if err != nil {
		return nil, err
	}

	return fiber.Map{
		"total":     total,
		"published": published,
		"drafts":    drafts,
		"views":     views,
	}, nil
}

func articleClassID(article *models.Article) uint {
	if article == nil || article.GradeLevel == nil || *article.GradeLevel == "" {
		return 0
	}

	id, err := strconv.ParseUint(*article.GradeLevel, 10, 64)
	if err != nil {
		return 0
	}

	return uint(id)
}
