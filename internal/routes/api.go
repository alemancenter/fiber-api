package routes

import (
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
	"github.com/alemancenter/fiber-api/internal/handlers/messages"
	"github.com/alemancenter/fiber-api/internal/handlers/notifications"
	"github.com/alemancenter/fiber-api/internal/handlers/posts"
	redisHandler "github.com/alemancenter/fiber-api/internal/handlers/redis"
	"github.com/alemancenter/fiber-api/internal/handlers/roles"
	"github.com/alemancenter/fiber-api/internal/handlers/security"
	"github.com/alemancenter/fiber-api/internal/handlers/settings"
	"github.com/alemancenter/fiber-api/internal/handlers/sitemap"
	"github.com/alemancenter/fiber-api/internal/handlers/users"
	"github.com/alemancenter/fiber-api/internal/middleware"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/gofiber/fiber/v2"
	fiberCompress "github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/etag"
)

// Setup registers all API routes on the given Fiber app
func Setup(app *fiber.App) {
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

	// Initialize handlers
	dashboardHandler := dashboard.New(dashboardSvc)
	_ = dashboardHandler // avoid unused variable error
	authH := auth.New(authSvc)
	articleH := articles.New(articleSvc)
	postH := posts.New(postSvc)
	userH := users.New(userSvc)
	fileH := files.New(fileSvc)
	commentH := comments.New(commentSvc)
	categoryH := categories.New(categorySvc)
	gradeH := grades.New(gradeSvc, fileSvc)
	calendarH := calendar.New(calendarSvc)
	notifH := notifications.New(notificationSvc)
	msgH := messages.New(messageSvc)
	secH := security.New(securitySvc)
	settingsH := settings.New(settingSvc)
	sitemapH := sitemap.New(sitemapSvc)
	analyticsH := analytics.New(analyticsSvc)
	rolesH := roles.New(roleSvc)
	redisH := redisHandler.New(redisSvc)
	healthH := health.New(healthSvc)

	// Global middleware
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.CORS())
	app.Use(middleware.RequestLogger())
	app.Use(fiberCompress.New())
	app.Use(etag.New())

	// Health check (no auth required)
	app.Get("/api/ping", healthH.Ping)
	app.Get("/api/health", healthH.Health)

	// API group with frontend guard and IP guard
	api := app.Group("/api",
		middleware.IPGuard(),
		middleware.FrontendGuard(),
	)

	// =====================
	// AUTH ROUTES
	// =====================
	authGroup := api.Group("/auth")
	authGroup.Post("/register", authH.Register)
	authGroup.Post("/login", authH.Login)
	authGroup.Get("/google/redirect", authH.GoogleRedirect)
	authGroup.Get("/google/callback", authH.GoogleCallback)
	authGroup.Post("/google/token", authH.GoogleTokenLogin)
	authGroup.Post("/password/forgot", authH.ForgotPassword)
	authGroup.Post("/password/reset", authH.ResetPassword)
	authGroup.Get("/email/verify/:id/:hash", authH.VerifyEmail)

	// Authenticated auth routes
	authSecure := authGroup.Group("", middleware.Auth(), middleware.UpdateLastActivity())
	authSecure.Post("/logout", authH.Logout)
	authSecure.Get("/user", authH.Me)
	authSecure.Put("/profile", authH.UpdateProfile)
	authSecure.Post("/email/resend", authH.ResendVerification)
	authSecure.Post("/account/delete", authH.DeleteAccount)
	authSecure.Post("/push-token", authH.RegisterPushToken)
	authSecure.Delete("/push-token", authH.DeletePushToken)

	// =====================
	// PUBLIC CONTENT ROUTES
	// =====================
	public := api.Group("", middleware.OptionalAuth(), middleware.TrackVisitor())

	// Articles
	public.Get("/articles", articleH.List)
	public.Get("/articles/by-class/:grade_level", articleH.ByClass)
	public.Get("/articles/by-keyword/:keyword", articleH.ByKeyword)
	public.Get("/articles/:id", articleH.Show)
	public.Get("/articles/file/:id/download", articleH.DownloadFile)

	// Files
	public.Get("/files/:id/info", fileH.Info)
	public.Post("/files/:id/increment-view", fileH.IncrementView)

	// Categories
	public.Get("/categories", categoryH.List)
	public.Get("/categories/:id", categoryH.Show)

	// Posts
	public.Get("/posts", postH.List)
	public.Get("/posts/:id", postH.Show)
	public.Post("/posts/:id/increment-view", postH.IncrementView)

	// Comments
	public.Get("/comments/:database", commentH.List)

	// Keywords
	public.Get("/keywords", gradeH.ListSchoolClasses) // handled by keyword handler
	public.Get("/keywords/:keyword", keywordHandlerShow(gradeH))

	// School classes & filter
	public.Get("/school-classes", gradeH.ListSchoolClasses)
	public.Get("/school-classes/:id", gradeH.GetSchoolClass)
	public.Get("/filter", gradeH.FilterMeta)
	public.Get("/filter/subjects/:classId", gradeH.ListSubjects)
	public.Get("/filter/semesters/:subjectId", gradeH.ListSemesters)

	// Grades
	public.Get("/grades", gradeH.ListSchoolClasses)
	public.Get("/grades/:id", gradeH.GetSchoolClass)
	public.Get("/grades/subjects/:id", gradeH.ListSubjects)
	public.Get("/grades/articles/:id", gradeH.GetGradeArticles)
	public.Get("/grades/files/:id/download", gradeH.DownloadGradeFile)

	// Home & Calendar
	public.Get("/home", homeHandler())
	public.Get("/home/calendar", calendarH.PublicEvents)
	public.Get("/home/event/:id", calendarH.PublicEventDetail)

	// =====================
	// FRONT PAGE ROUTES
	// =====================
	front := api.Group("/front", middleware.OptionalAuth())
	front.Get("/settings", settingsH.GetPublic)

	// Legal
	legal := api.Group("/legal")
	legal.Get("/privacy-policy", legalHandler("privacy-policy"))
	legal.Get("/terms-of-service", legalHandler("terms-of-service"))
	legal.Get("/cookie-policy", legalHandler("cookie-policy"))
	legal.Get("/disclaimer", legalHandler("disclaimer"))

	// Language
	langGroup := api.Group("/lang")
	langGroup.Post("/change", langChangeHandler())
	langGroup.Get("/current", langCurrentHandler())

	// =====================
	// AUTHENTICATED USER ROUTES
	// =====================
	userRoutes := api.Group("", middleware.Auth(), middleware.UpdateLastActivity())

	// Reactions
	userRoutes.Post("/reactions", commentH.CreateReaction)
	userRoutes.Delete("/reactions/:comment_id", commentH.DeleteReaction)
	userRoutes.Get("/reactions/:comment_id", commentH.GetReactions)

	// Roles (view only)
	userRoutes.Get("/roles", rolesH.ListRoles)
	userRoutes.Get("/roles/:id", rolesH.GetRole)

	// File upload
	userRoutes.Post("/upload/image", fileH.UploadImage)
	userRoutes.Post("/upload/file", fileH.UploadDocument)

	// AI generation
	userRoutes.Post("/ai/generate", aiGenerateHandler())

	// =====================
	// DASHBOARD ROUTES
	// =====================
	dash := api.Group("/dashboard",
		middleware.Auth(),
		middleware.UpdateLastActivity(),
		middleware.DashboardSecurityHeaders(),
	)

	// Dashboard home
	dash.Get("", dashboardHandler.Home)
	dash.Get("/content-analytics", analyticsH.ContentAnalytics)

	// Activities
	dash.Get("/activities", dashboardHandler.Activities)
	dash.Delete("/activities/clean", dashboardHandler.CleanActivities)

	// Visitor Analytics (requires monitoring permission)
	dashMonitor := dash.Group("", middleware.Can("manage monitoring"))
	dashMonitor.Get("/visitor-analytics", analyticsH.VisitorAnalytics)
	dashMonitor.Post("/visitor-analytics/prune", analyticsH.PruneAnalytics)
	dashMonitor.Get("/performance/summary", analyticsH.PerformanceSummary)

	// Articles management
	dashArticles := dash.Group("/articles", middleware.Can("manage articles"))
	dashArticles.Get("/stats", articleH.DashboardStats)
	dashArticles.Get("", articleH.DashboardList)
	dashArticles.Get("/create", articleH.DashboardCreateData)
	dashArticles.Post("", articleH.DashboardCreate)
	dashArticles.Get("/:id/edit", articleH.DashboardEditData)
	dashArticles.Get("/:id", articleH.Show)
	dashArticles.Put("/:id", articleH.DashboardUpdate)
	dashArticles.Delete("/:id", articleH.DashboardDelete)
	dashArticles.Post("/:id/publish", articleH.DashboardPublish)
	dashArticles.Post("/:id/unpublish", articleH.DashboardUnpublish)

	// AI (dashboard)
	dash.Post("/ai/generate", aiGenerateHandler())

	// School classes
	dash.Get("/school-classes", gradeH.DashboardListSchoolClasses)
	dash.Post("/school-classes", gradeH.DashboardCreateSchoolClass)
	dash.Get("/school-classes/:id", gradeH.GetSchoolClass)
	dash.Put("/school-classes/:id", gradeH.DashboardUpdateSchoolClass)
	dash.Delete("/school-classes/:id", gradeH.DashboardDeleteSchoolClass)

	// Semesters
	dash.Get("/semesters", gradeH.DashboardListSemesters)
	dash.Post("/semesters", gradeH.DashboardCreateSemester)
	dash.Get("/semesters/:id", semesterShowHandler(gradeH))
	dash.Put("/semesters/:id", gradeH.DashboardUpdateSemester)
	dash.Delete("/semesters/:id", gradeH.DashboardDeleteSemester)

	// Subjects
	dash.Get("/subjects", gradeH.DashboardListSubjects)
	dash.Post("/subjects", gradeH.DashboardCreateSubject)
	dash.Get("/subjects/:id", subjectShowHandler(gradeH))
	dash.Put("/subjects/:id", subjectUpdateHandler(gradeH))
	dash.Delete("/subjects/:id", subjectDeleteHandler(gradeH))

	// Roles & Permissions
	dashRoles := dash.Group("", middleware.Can("manage roles"))
	dashRoles.Get("/roles", rolesH.ListRoles)
	dashRoles.Post("/roles", rolesH.CreateRole)
	dashRoles.Get("/roles/:id", rolesH.GetRole)
	dashRoles.Put("/roles/:id", rolesH.UpdateRole)
	dashRoles.Delete("/roles/:id", rolesH.DeleteRole)
	dashRoles.Get("/permissions", rolesH.ListPermissions)
	dashRoles.Post("/permissions", rolesH.CreatePermission)
	dashRoles.Put("/permissions/:id", rolesH.UpdatePermission)
	dashRoles.Delete("/permissions/:id", rolesH.DeletePermission)

	// Users management
	dashUsers := dash.Group("", middleware.Can("manage users"))
	dashUsers.Get("/users/search", userH.Search)
	dashUsers.Get("/users", userH.List)
	dashUsers.Post("/users", userH.Create)
	dashUsers.Get("/users/:user", userH.Show)
	dashUsers.Put("/users/:user", userH.Update)
	dashUsers.Put("/users/:user/roles-permissions", userH.UpdateRolesPermissions)
	dashUsers.Delete("/users/:user", userH.Delete)
	dashUsers.Post("/users/bulk-delete", userH.BulkDelete)
	dashUsers.Post("/users/update-status", userH.UpdateStatus)

	// Settings
	dashSettings := dash.Group("", middleware.Can("manage settings"))
	dashSettings.Get("/settings", settingsH.GetAll)
	dashSettings.Post("/settings", settingsH.Update)
	dashSettings.Post("/settings/update", settingsH.Update)
	dashSettings.Post("/settings/smtp/test", settingsH.TestSMTP)
	dashSettings.Post("/settings/smtp/send-test", settingsH.SendTestEmail)
	dashSettings.Post("/settings/robots", settingsH.UpdateRobots)

	// Sitemap
	dashSitemap := dash.Group("", middleware.Can("manage sitemap"))
	dashSitemap.Get("/sitemap/status", sitemapH.Status)
	dashSitemap.Post("/sitemap/generate", sitemapH.GenerateAll)
	dashSitemap.Delete("/sitemap/delete/:type/:database", sitemapH.Delete)

	// Security
	dashSecurity := dash.Group("/security")
	dashSecurity.Get("/stats", secH.Stats)
	dashSecurity.Get("/logs", secH.Logs)
	dashSecurity.Post("/logs/:id/resolve", secH.ResolveLog)
	dashSecurity.Delete("/logs/:id", secH.DeleteLog)
	dashSecurity.Delete("/logs", secH.DeleteAllLogs)
	dashSecurity.Get("/analytics", secH.Analytics)
	dashSecurity.Get("/analytics/routes", secH.TopRoutes)
	dashSecurity.Get("/analytics/geo", secH.GeoDistribution)
	dashSecurity.Get("/ip/:ip", secH.IPDetails)
	dashSecurity.Post("/ip/block", secH.BlockIP)
	dashSecurity.Post("/ip/unblock", secH.UnblockIP)
	dashSecurity.Post("/ip/trust", secH.TrustIP)
	dashSecurity.Post("/ip/untrust", secH.UntrustIP)
	dashSecurity.Get("/blocked-ips", secH.BlockedIPs)
	dashSecurity.Get("/trusted-ips", secH.TrustedIPs)
	dashSecurity.Get("/overview", secH.Overview)

	// Blocked IPs shortcut
	dashBlockedIPs := dash.Group("/blocked-ips", middleware.Can("manage security"))
	dashBlockedIPs.Get("", secH.BlockedIPs)
	dashBlockedIPs.Post("", secH.BlockIP)
	dashBlockedIPs.Delete("/:id", blockedIPDeleteHandler())
	dashBlockedIPs.Delete("/bulk", blockedIPBulkDeleteHandler())

	// Trusted IPs shortcut
	dashTrustedIPs := dash.Group("/trusted-ips", middleware.Can("manage security"))
	dashTrustedIPs.Get("", secH.TrustedIPs)
	dashTrustedIPs.Post("", secH.TrustIP)
	dashTrustedIPs.Post("/check", trustedIPCheckHandler())
	dashTrustedIPs.Delete("/:trustedIp", trustedIPDeleteHandler())

	// Calendar
	dash.Get("/calendar/databases", calendarH.Databases)
	dash.Get("/calendar/events", calendarH.GetEvents)
	dash.Post("/calendar/events", calendarH.CreateEvent)
	dash.Put("/calendar/events/:id", calendarH.UpdateEvent)
	dash.Delete("/calendar/events/:id", calendarH.DeleteEvent)

	// Messages
	dash.Get("/messages/inbox", msgH.Inbox)
	dash.Get("/messages/sent", msgH.Sent)
	dash.Get("/messages/drafts", msgH.Drafts)
	dash.Post("/messages/send", msgH.Send)
	dash.Post("/messages/draft", msgH.Draft)
	dash.Get("/messages/:id", msgH.Get)
	dash.Post("/messages/:id/read", msgH.MarkAsRead)
	dash.Post("/messages/:id/important", msgH.ToggleImportant)
	dash.Delete("/messages/:id", msgH.Delete)

	// Files management
	dash.Get("/files", fileH.DashboardList)
	dash.Post("/files", fileH.DashboardUpload)
	dash.Get("/files/:id", fileH.DashboardShow)
	dash.Get("/files/:id/info", fileH.Info)
	dash.Get("/files/:id/download", fileH.DashboardDownload)
	dash.Put("/files/:id", fileH.DashboardUpdate)
	dash.Delete("/files/:id", fileH.DashboardDelete)

	// Notifications
	dash.Get("/notifications", notifH.List)
	dash.Post("/notifications", notifH.Create)
	dash.Get("/notifications/latest", notifH.Latest)
	dash.Post("/notifications/:id/read", notifH.MarkAsRead)
	dash.Post("/notifications/read-all", notifH.MarkAllRead)
	dash.Post("/notifications/bulk", notifH.BulkAction)
	dash.Post("/notifications/prune", notifH.Prune)
	dash.Delete("/notifications/:id", notifH.Delete)

	// Redis management (admin only)
	dashRedis := dash.Group("/redis", middleware.AdminOnly())
	dashRedis.Get("/keys", redisH.ListKeys)
	dashRedis.Post("", redisH.SetKey)
	dashRedis.Delete("/expired/clean", redisH.CleanExpired)
	dashRedis.Get("/test", redisH.TestConnection)
	dashRedis.Get("/info", redisH.GetInfo)
	dashRedis.Delete("/:key", redisH.DeleteKey)

	// Categories (dashboard)
	dash.Get("/categories", categoryH.DashboardList)
	dash.Post("/categories", categoryH.DashboardCreate)
	dash.Get("/categories/:id", categoryH.DashboardShow)
	dash.Post("/categories/:id/update", categoryH.DashboardUpdate)
	dash.Delete("/categories/:id", categoryH.DashboardDelete)
	dash.Post("/categories/:id/toggle", categoryH.DashboardToggleStatus)

	// Comments (dashboard)
	dash.Get("/comments/:database", commentH.DashboardList)
	dash.Post("/comments/:database", commentH.DashboardCreate)
	dash.Delete("/comments/:database/:id", commentH.DashboardDelete)

	// Posts (dashboard)
	dash.Post("/posts", postH.DashboardCreate)
	dash.Post("/posts/:id", postH.DashboardUpdate)
	dash.Post("/posts/:id/toggle-status", postH.DashboardToggleStatus)
	dash.Delete("/posts/:id", postH.DashboardDelete)

	// Secure file upload (dashboard)
	dash.Post("/secure/upload-image", fileH.SecureUploadImage)
	dash.Post("/secure/upload-document", fileH.SecureUploadDocument)

	// Secure file view
	api.Get("/secure/view", middleware.Auth(), fileH.SecureView)
}

// --- Inline mini-handlers for simple endpoints ---

func homeHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "مرحباً بك في Alemancenter API",
			"version": "2.0.0",
		})
	}
}

func legalHandler(page string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"page":    page,
			"content": "",
		})
	}
}

func langChangeHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		type LangRequest struct {
			Locale string `json:"locale"`
		}
		var req LangRequest
		c.BodyParser(&req)
		return c.JSON(fiber.Map{"success": true, "locale": req.Locale})
	}
}

func langCurrentHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		lang := c.Get("Accept-Language", "ar")
		if len(lang) >= 2 {
			lang = lang[:2]
		}
		return c.JSON(fiber.Map{"success": true, "locale": lang})
	}
}

func aiGenerateHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "AI generation endpoint - configure Together AI API key",
		})
	}
}

func blockedIPDeleteHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	}
}

func blockedIPBulkDeleteHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	}
}

func trustedIPCheckHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	}
}

func trustedIPDeleteHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	}
}

func semesterShowHandler(h *grades.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	}
}

func subjectShowHandler(h *grades.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	}
}

func subjectUpdateHandler(h *grades.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	}
}

func subjectDeleteHandler(h *grades.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	}
}

func keywordHandlerShow(h *grades.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	}
}
