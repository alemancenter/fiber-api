package routes

import (
	"github.com/alemancenter/fiber-api/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func RegisterDashboardRoutes(api fiber.Router, h *Handlers) {
	// =====================
	// DASHBOARD ROUTES
	// =====================
	dash := api.Group("/dashboard",
		middleware.Auth(),
		middleware.UpdateLastActivity(),
		middleware.DashboardSecurityHeaders(),
	)

	// Dashboard home
	dash.Get("", h.Analytics.DashboardSummary)
	dash.Get("/content-analytics", h.Analytics.ContentAnalytics)

	// Activities
	dash.Get("/activities", h.Dashboard.Activities)
	dash.Delete("/activities/clean", h.Dashboard.CleanActivities)

	// Visitor Analytics (requires monitoring permission)
	dashMonitor := dash.Group("", middleware.Can("manage monitoring"))
	dashMonitor.Get("/visitor-analytics", h.Analytics.VisitorAnalytics)
	dashMonitor.Post("/visitor-analytics/prune", h.Analytics.PruneAnalytics)
	dashMonitor.Get("/performance/summary", h.Analytics.PerformanceSummary)

	// Articles management
	dashArticles := dash.Group("/articles", middleware.Can("manage articles"))
	dashArticles.Get("/stats", h.Articles.DashboardStats)
	dashArticles.Get("", h.Articles.DashboardList)
	dashArticles.Get("/create", h.Articles.DashboardCreateData)
	dashArticles.Post("", h.Articles.DashboardCreate)
	dashArticles.Get("/:id/edit", h.Articles.DashboardEditData)
	dashArticles.Get("/:id", h.Articles.Show)
	dashArticles.Put("/:id", h.Articles.DashboardUpdate)
	dashArticles.Delete("/:id", h.Articles.DashboardDelete)
	dashArticles.Post("/:id/publish", h.Articles.DashboardPublish)
	dashArticles.Post("/:id/unpublish", h.Articles.DashboardUnpublish)

	// AI (dashboard)
	dash.Post("/ai/generate", h.AI.Generate)

	// School classes
	dash.Get("/school-classes", h.Grades.DashboardListSchoolClasses)
	dash.Post("/school-classes", h.Grades.DashboardCreateSchoolClass)
	dash.Get("/school-classes/:id", h.Grades.GetSchoolClass)
	dash.Put("/school-classes/:id", h.Grades.DashboardUpdateSchoolClass)
	dash.Delete("/school-classes/:id", h.Grades.DashboardDeleteSchoolClass)

	// Semesters
	dash.Get("/semesters", h.Grades.DashboardListSemesters)
	dash.Post("/semesters", h.Grades.DashboardCreateSemester)
	dash.Get("/semesters/:id", h.Grades.DashboardGetSemester)
	dash.Put("/semesters/:id", h.Grades.DashboardUpdateSemester)
	dash.Delete("/semesters/:id", h.Grades.DashboardDeleteSemester)

	// Subjects
	dash.Get("/subjects", h.Grades.DashboardListSubjects)
	dash.Post("/subjects", h.Grades.DashboardCreateSubject)
	dash.Get("/subjects/:id", h.Grades.DashboardGetSubject)
	dash.Put("/subjects/:id", h.Grades.DashboardUpdateSubject)
	dash.Delete("/subjects/:id", h.Grades.DashboardDeleteSubject)

	// Roles & Permissions
	dashRoles := dash.Group("", middleware.Can("manage roles"))
	dashRoles.Get("/roles", h.Roles.ListRoles)
	dashRoles.Post("/roles", h.Roles.CreateRole)
	dashRoles.Get("/roles/:id", h.Roles.GetRole)
	dashRoles.Put("/roles/:id", h.Roles.UpdateRole)
	dashRoles.Delete("/roles/:id", h.Roles.DeleteRole)
	dashRoles.Get("/permissions", h.Roles.ListPermissions)
	dashRoles.Post("/permissions", h.Roles.CreatePermission)
	dashRoles.Put("/permissions/:id", h.Roles.UpdatePermission)
	dashRoles.Delete("/permissions/:id", h.Roles.DeletePermission)

	// Users management
	dashUsers := dash.Group("", middleware.Can("manage users"))
	dashUsers.Get("/users/search", h.Users.Search)
	dashUsers.Get("/users", h.Users.List)
	dashUsers.Post("/users", h.Users.Create)
	dashUsers.Post("/users/bulk-delete", h.Users.BulkDelete)
	dashUsers.Post("/users/update-status", h.Users.UpdateStatus)
	dashUsers.Get("/users/:user", h.Users.Show)
	dashUsers.Put("/users/:user", h.Users.Update)
	dashUsers.Put("/users/:user/roles-permissions", h.Users.UpdateRolesPermissions)
	dashUsers.Delete("/users/:user", h.Users.Delete)

	// Settings
	dashSettings := dash.Group("", middleware.Can("manage settings"))
	dashSettings.Get("/settings", h.Settings.GetAll)
	dashSettings.Post("/settings", h.Settings.Update)
	dashSettings.Post("/settings/update", h.Settings.Update)
	dashSettings.Post("/settings/smtp/test", h.Settings.TestSMTP)
	dashSettings.Post("/settings/smtp/send-test", h.Settings.SendTestEmail)
	dashSettings.Post("/settings/robots", h.Settings.UpdateRobots)

	// Sitemap
	dashSitemap := dash.Group("", middleware.Can("manage sitemap"))
	dashSitemap.Get("/sitemap/status", h.Sitemap.Status)
	dashSitemap.Post("/sitemap/generate", h.Sitemap.GenerateAll)
	dashSitemap.Delete("/sitemap/delete/:type/:database", h.Sitemap.Delete)

	// Security
	dashSecurity := dash.Group("/security", middleware.Can("manage security"))
	dashSecurity.Get("/stats", h.Security.Stats)
	dashSecurity.Get("/logs", h.Security.Logs)
	dashSecurity.Delete("/logs", h.Security.DeleteAllLogs)
	dashSecurity.Post("/logs/:id/resolve", h.Security.ResolveLog)
	dashSecurity.Delete("/logs/:id", h.Security.DeleteLog)
	dashSecurity.Get("/analytics", h.Security.Analytics)
	dashSecurity.Get("/analytics/routes", h.Security.TopRoutes)
	dashSecurity.Get("/analytics/geo", h.Security.GeoDistribution)
	dashSecurity.Get("/ip/:ip", h.Security.IPDetails)
	dashSecurity.Post("/ip/block", h.Security.BlockIP)
	dashSecurity.Post("/ip/unblock", h.Security.UnblockIP)
	dashSecurity.Post("/ip/trust", h.Security.TrustIP)
	dashSecurity.Post("/ip/untrust", h.Security.UntrustIP)
	dashSecurity.Get("/blocked-ips", h.Security.BlockedIPs)
	dashSecurity.Get("/trusted-ips", h.Security.TrustedIPs)
	dashSecurity.Get("/overview", h.Security.Overview)

	// Blocked IPs shortcut
	dashBlockedIPs := dash.Group("/blocked-ips", middleware.Can("manage security"))
	dashBlockedIPs.Get("", h.Security.BlockedIPs)
	dashBlockedIPs.Post("", h.Security.BlockIP)
	dashBlockedIPs.Delete("/:ip", h.Security.UnblockIP)

	// Trusted IPs shortcut
	dashTrustedIPs := dash.Group("/trusted-ips", middleware.Can("manage security"))
	dashTrustedIPs.Get("", h.Security.TrustedIPs)
	dashTrustedIPs.Post("", h.Security.TrustIP)
	dashTrustedIPs.Delete("/:ip", h.Security.UntrustIP)

	// Calendar
	dash.Get("/calendar/databases", h.Calendar.Databases)
	dash.Get("/calendar/events", h.Calendar.GetEvents)
	dash.Post("/calendar/events", h.Calendar.CreateEvent)
	dash.Put("/calendar/events/:id", h.Calendar.UpdateEvent)
	dash.Delete("/calendar/events/:id", h.Calendar.DeleteEvent)

	// Messages
	dash.Get("/messages/inbox", h.Messages.Inbox)
	dash.Get("/messages/sent", h.Messages.Sent)
	dash.Get("/messages/drafts", h.Messages.Drafts)
	dash.Post("/messages/send", h.Messages.Send)
	dash.Post("/messages/draft", h.Messages.Draft)
	dash.Get("/messages/:id", h.Messages.Get)
	dash.Post("/messages/:id/read", h.Messages.MarkAsRead)
	dash.Post("/messages/:id/important", h.Messages.ToggleImportant)
	dash.Delete("/messages/:id", h.Messages.Delete)

	// Files management
	dash.Get("/files", h.Files.DashboardList)
	dash.Post("/files", h.Files.DashboardUpload)
	dash.Get("/files/:id/info", h.Files.Info)
	dash.Get("/files/:id/download", h.Files.DashboardDownload)
	dash.Get("/files/:id", h.Files.DashboardShow)
	dash.Put("/files/:id", h.Files.DashboardUpdate)
	dash.Delete("/files/:id", h.Files.DashboardDelete)

	// Notifications
	dash.Get("/notifications", h.Notifications.List)
	dash.Post("/notifications", h.Notifications.Create)
	dash.Get("/notifications/latest", h.Notifications.Latest)
	dash.Post("/notifications/read-all", h.Notifications.MarkAllRead)
	dash.Post("/notifications/bulk", h.Notifications.BulkAction)
	dash.Post("/notifications/prune", h.Notifications.Prune)
	dash.Post("/notifications/:id/read", h.Notifications.MarkAsRead)
	dash.Delete("/notifications/:id", h.Notifications.Delete)

	// Categories (dashboard)
	dash.Get("/categories", h.Categories.DashboardList)
	dash.Post("/categories", h.Categories.DashboardCreate)
	dash.Get("/categories/:id", h.Categories.DashboardShow)
	dash.Post("/categories/:id/update", h.Categories.DashboardUpdate)
	dash.Post("/categories/:id/toggle", h.Categories.DashboardToggleStatus)
	dash.Delete("/categories/:id", h.Categories.DashboardDelete)

	// Comments (dashboard)
	dash.Get("/comments/:database", h.Comments.DashboardList)
	dash.Post("/comments/:database", h.Comments.DashboardCreate)
	dash.Delete("/comments/:database/:id", h.Comments.DashboardDelete)

	// Posts (dashboard)
	dash.Post("/posts", h.Posts.DashboardCreate)
	dash.Post("/posts/:id/toggle-status", h.Posts.DashboardToggleStatus)
	dash.Post("/posts/:id", h.Posts.DashboardUpdate)
	dash.Delete("/posts/:id", h.Posts.DashboardDelete)

	// Secure file upload (dashboard)
	dash.Post("/secure/upload-image", h.Files.SecureUploadImage)
	dash.Post("/secure/upload-document", h.Files.SecureUploadDocument)
}
