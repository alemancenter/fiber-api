package routes

import (
	"github.com/alemancenter/fiber-api/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func RegisterAuthRoutes(api fiber.Router, h *Handlers) {
	// =====================
	// AUTH ROUTES
	// =====================
	authGroup := api.Group("/auth")

	// Brute-force-sensitive endpoints get a dedicated per-IP rate limiter
	authGroup.Post("/register", middleware.AuthRateLimit(), h.Auth.Register)
	authGroup.Post("/login", middleware.AuthRateLimit(), h.Auth.Login)
	authGroup.Post("/refresh", middleware.AuthRateLimit(), h.Auth.RefreshToken)
	authGroup.Post("/password/forgot", middleware.AuthRateLimit(), h.Auth.ForgotPassword)

	authGroup.Get("/google/redirect", h.Auth.GoogleRedirect)
	authGroup.Get("/google/callback", h.Auth.GoogleCallback)
	authGroup.Post("/google/token", h.Auth.GoogleTokenLogin)
	authGroup.Post("/password/reset", h.Auth.ResetPassword)
	authGroup.Get("/email/verify/:id/:hash", h.Auth.VerifyEmail)

	// Authenticated auth routes
	authSecure := authGroup.Group("", middleware.Auth(), middleware.UpdateLastActivity())
	authSecure.Post("/logout", h.Auth.Logout)
	authSecure.Get("/user", h.Auth.Me)
	authSecure.Put("/profile", h.Auth.UpdateProfile)
	authSecure.Post("/email/resend", h.Auth.ResendVerification)
	authSecure.Post("/account/delete", h.Auth.DeleteAccount)
	authSecure.Post("/push-token", h.Auth.RegisterPushToken)
	authSecure.Delete("/push-token", h.Auth.DeletePushToken)

	// =====================
	// AUTHENTICATED USER ROUTES
	// =====================
	userRoutes := api.Group("", middleware.Auth(), middleware.UpdateLastActivity())

	// Reactions
	userRoutes.Post("/reactions", h.Comments.CreateReaction)
	userRoutes.Delete("/reactions/:comment_id", h.Comments.DeleteReaction)
	userRoutes.Get("/reactions/:comment_id", h.Comments.GetReactions)

	// Roles (view only)
	userRoutes.Get("/roles", h.Roles.ListRoles)
	userRoutes.Get("/roles/:id", h.Roles.GetRole)

	// File upload
	userRoutes.Post("/upload/image", h.Files.UploadImage)
	userRoutes.Post("/upload/file", h.Files.UploadDocument)

	// Secure file view
	userRoutes.Get("/secure/view", h.Files.SecureView)

	// AI generation
	userRoutes.Post("/ai/generate", h.AI.Generate)
}
