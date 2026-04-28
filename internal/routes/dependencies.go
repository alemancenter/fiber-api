package routes

import (
	"github.com/alemancenter/fiber-api/internal/handlers/ai"
	"github.com/alemancenter/fiber-api/internal/handlers/analytics"
	"github.com/alemancenter/fiber-api/internal/handlers/articles"
	"github.com/alemancenter/fiber-api/internal/handlers/auth"
	"github.com/alemancenter/fiber-api/internal/handlers/calendar"
	"github.com/alemancenter/fiber-api/internal/handlers/categories"
	"github.com/alemancenter/fiber-api/internal/handlers/comments"
	"github.com/alemancenter/fiber-api/internal/handlers/dashboard"
	"github.com/alemancenter/fiber-api/internal/handlers/files"
	"github.com/alemancenter/fiber-api/internal/handlers/grades"
	"github.com/alemancenter/fiber-api/internal/handlers/health"
	"github.com/alemancenter/fiber-api/internal/handlers/keywords"
	"github.com/alemancenter/fiber-api/internal/handlers/messages"
	"github.com/alemancenter/fiber-api/internal/handlers/notifications"
	"github.com/alemancenter/fiber-api/internal/handlers/posts"
	redisHandler "github.com/alemancenter/fiber-api/internal/handlers/redis"
	"github.com/alemancenter/fiber-api/internal/handlers/roles"
	"github.com/alemancenter/fiber-api/internal/handlers/security"
	"github.com/alemancenter/fiber-api/internal/handlers/settings"
	"github.com/alemancenter/fiber-api/internal/handlers/sitemap"
	"github.com/alemancenter/fiber-api/internal/handlers/users"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/services"
)

type Handlers struct {
	Dashboard     *dashboard.Handler
	Auth          *auth.Handler
	Articles      *articles.Handler
	Posts         *posts.Handler
	Users         *users.Handler
	Files         *files.Handler
	Comments      *comments.Handler
	Categories    *categories.Handler
	Grades        *grades.Handler
	Calendar      *calendar.Handler
	Notifications *notifications.Handler
	Messages      *messages.Handler
	Security      *security.Handler
	Settings      *settings.Handler
	Sitemap       *sitemap.Handler
	Analytics     *analytics.Handler
	Roles         *roles.Handler
	Redis         *redisHandler.Handler
	Health        *health.Handler
	Keywords      *keywords.Handler
	AI            *ai.Handler
}

func NewDependencies() *Handlers {
	// Initialize Dependencies
	fileRepo := repositories.NewFileRepository()
	fileSvc := services.NewFileService(fileRepo)
	articleRepo := repositories.NewArticleRepository()
	articleSvc := services.NewArticleService(articleRepo, fileSvc)

	userRepo := repositories.NewUserRepository()
	jwtSvc := services.NewJWTService()
	mailSvc := services.NewMailService()
	authSvc := services.NewAuthService(userRepo, jwtSvc, mailSvc)
	userSvc := services.NewUserService(userRepo)

	categoryRepo := repositories.NewCategoryRepository()
	categorySvc := services.NewCategoryService(categoryRepo)

	commentRepo := repositories.NewCommentRepository()
	commentSvc := services.NewCommentService(commentRepo)
	postRepo := repositories.NewPostRepository()
	postSvc := services.NewPostService(postRepo)

	gradeRepo := repositories.NewGradeRepository()
	gradeSvc := services.NewGradeService(gradeRepo)

	calendarRepo := repositories.NewCalendarRepository()
	calendarSvc := services.NewCalendarService(calendarRepo)

	dashboardRepo := repositories.NewDashboardRepository()
	dashboardSvc := services.NewDashboardService(dashboardRepo)

	healthRepo := repositories.NewHealthRepository()
	healthSvc := services.NewHealthService(healthRepo)

	messageRepo := repositories.NewMessageRepository()
	messageSvc := services.NewMessageService(messageRepo)

	notificationRepo := repositories.NewNotificationRepository()
	notificationSvc := services.NewNotificationService(notificationRepo)

	redisRepo := repositories.NewRedisRepository()
	redisSvc := services.NewRedisService(redisRepo)

	roleRepo := repositories.NewRoleRepository()
	roleSvc := services.NewRoleService(roleRepo)

	securityRepo := repositories.NewSecurityRepository()
	securitySvc := services.NewSecurityService(securityRepo)

	settingRepo := repositories.NewSettingRepository()
	settingSvc := services.NewSettingService(settingRepo)

	sitemapRepo := repositories.NewSitemapRepository()
	sitemapSvc := services.NewSitemapService(sitemapRepo)

	analyticsRepo := repositories.NewAnalyticsRepository()
	analyticsSvc := services.NewAnalyticsService(analyticsRepo)

	keywordRepo := repositories.NewKeywordRepository()
	keywordSvc := services.NewKeywordService(keywordRepo)

	aiSvc := services.NewAIService()

	return &Handlers{
		Dashboard:     dashboard.New(dashboardSvc),
		Auth:          auth.New(authSvc),
		Articles:      articles.New(articleSvc),
		Posts:         posts.New(postSvc),
		Users:         users.New(userSvc),
		Files:         files.New(fileSvc),
		Comments:      comments.New(commentSvc),
		Categories:    categories.New(categorySvc),
		Grades:        grades.New(gradeSvc, fileSvc),
		Calendar:      calendar.New(calendarSvc),
		Notifications: notifications.New(notificationSvc),
		Messages:      messages.New(messageSvc),
		Security:      security.New(securitySvc),
		Settings:      settings.New(settingSvc),
		Sitemap:       sitemap.New(sitemapSvc),
		Analytics:     analytics.New(analyticsSvc),
		Roles:         roles.New(roleSvc),
		Redis:         redisHandler.New(redisSvc),
		Health:        health.New(healthSvc),
		Keywords:      keywords.New(keywordSvc),
		AI:            ai.New(aiSvc),
	}
}
