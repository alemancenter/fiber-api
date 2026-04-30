package services

import (
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/utils"
)

type HomeData struct {
	Articles      []models.Article     `json:"articles"`
	Posts         []models.Post        `json:"posts"`
	FeaturedPosts []models.Post        `json:"featured_posts"`
	Categories    []models.Category    `json:"categories"`
	Classes       []models.SchoolClass `json:"classes"`
}

type HomeService interface {
	GetHome(countryID database.CountryID) (*HomeData, error)
}

type homeService struct {
	articleRepo  repositories.ArticleRepository
	postRepo     repositories.PostRepository
	categoryRepo repositories.CategoryRepository
	gradeRepo    repositories.GradeRepository
	cache        CacheService
}

func NewHomeService(
	articleRepo repositories.ArticleRepository,
	postRepo repositories.PostRepository,
	categoryRepo repositories.CategoryRepository,
	gradeRepo repositories.GradeRepository,
	cache CacheService,
) HomeService {
	return &homeService{
		articleRepo:  articleRepo,
		postRepo:     postRepo,
		categoryRepo: categoryRepo,
		gradeRepo:    gradeRepo,
		cache:        cache,
	}
}

func (s *homeService) GetHome(countryID database.CountryID) (*HomeData, error) {
	cacheKey := utils.CacheKey("home", countryID)

	var cached HomeData
	if s.cache != nil && s.cache.Get(cacheKey, &cached) {
		return &cached, nil
	}

	data := &HomeData{}

	// Get Articles
	statusPublished := 1
	articles, _, err := s.articleRepo.List(countryID, utils.Pagination{Page: 1, PerPage: 6}, &models.ArticleFilter{
		Status: &statusPublished,
	})
	if err != nil {
		return nil, MapError(err)
	}
	data.Articles = articles

	// Get Posts
	postsFilter := &models.PostFilter{
		Search: "",
	}
	// Need to check how ListPaginated filters by active, usually it's handled in the repo
	posts, _, err := s.postRepo.ListPaginated(countryID, postsFilter, 6, 0)
	if err != nil {
		return nil, MapError(err)
	}
	data.Posts = posts

	// Get Featured Posts
	featuredFilter := &models.PostFilter{
		Featured: "true",
	}
	featuredPosts, _, err := s.postRepo.ListPaginated(countryID, featuredFilter, 6, 0)
	if err != nil {
		return nil, MapError(err)
	}
	data.FeaturedPosts = featuredPosts

	// Get Categories
	categories, err := s.categoryRepo.FindAllActive(countryID)
	if err != nil {
		return nil, MapError(err)
	}
	data.Categories = categories

	// Get School Classes
	classes, err := s.gradeRepo.ListSchoolClasses(countryID)
	if err != nil {
		return nil, MapError(err)
	}
	data.Classes = classes

	if s.cache != nil {
		_ = s.cache.Set(cacheKey, data, 10*time.Minute)
	}

	return data, nil
}
