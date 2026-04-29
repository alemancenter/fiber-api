package routes

import (
	"github.com/alemancenter/fiber-api/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func RegisterPublicRoutes(api fiber.Router, h *Handlers) {
	// =====================
	// PUBLIC CONTENT ROUTES
	// =====================
	public := api.Group("", middleware.OptionalAuth(), middleware.TrackVisitor())

	// Articles
	public.Get("/articles", h.Articles.List)
	public.Get("/articles/by-class/:grade_level", h.Articles.ByClass)
	public.Get("/articles/by-keyword/:keyword", h.Articles.ByKeyword)
	public.Get("/articles/:id", h.Articles.Show)
	public.Get("/articles/file/:id/download", h.Articles.DownloadFile)
	public.Get("/articles/file/:id/download-url", h.Articles.GetDownloadToken)
	public.Get("/articles/download", h.Articles.DownloadFileSigned)

	// Files
	public.Get("/files/:id/info", h.Files.Info)
	public.Post("/files/:id/increment-view", h.Files.IncrementView)

	// Categories
	public.Get("/categories", h.Categories.List)
	public.Get("/categories/:id", h.Categories.Show)

	// Posts
	public.Get("/posts", h.Posts.List)
	public.Get("/posts/:id", h.Posts.Show)
	public.Post("/posts/:id/increment-view", h.Posts.IncrementView)

	// Comments
	public.Get("/comments/:database", h.Comments.List)

	// Keywords
	public.Get("/keywords", h.Keywords.Index)
	public.Get("/keywords/:keyword", h.Keywords.Show)

	// School classes & filter (Aliases to Grades)
	public.Get("/school-classes", h.Grades.ListSchoolClasses)
	public.Get("/school-classes/:id", h.Grades.GetSchoolClass)
	public.Get("/filter", h.Grades.FilterMeta)
	public.Get("/filter/subjects/:classId", h.Grades.ListSubjects)
	public.Get("/filter/semesters/:subjectId", h.Grades.ListSemesters)

	// Grades (Aliases to School Classes - maintained for frontend backward compatibility)
	public.Get("/grades", h.Grades.ListSchoolClasses)
	public.Get("/grades/:id", h.Grades.GetSchoolClass)
	public.Get("/grades/subjects/:classId", h.Grades.ListSubjects)
	public.Get("/grades/articles/:id", h.Grades.GetGradeArticles)
	public.Get("/grades/files/:id/download", h.Grades.DownloadGradeFile)

	// Home & Calendar
	public.Get("/home", homeHandler())
	public.Get("/home/calendar", h.Calendar.PublicEvents)
	public.Get("/home/event/:id", h.Calendar.PublicEventDetail)

	// =====================
	// FRONT PAGE ROUTES
	// =====================
	front := api.Group("/front", middleware.OptionalAuth())
	front.Get("/settings", h.Settings.GetPublic)

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
}
