package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/middleware"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/internal/routes"
	"github.com/alemancenter/fiber-api/internal/services"
	contentauditService "github.com/alemancenter/fiber-api/internal/services/contentaudit"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/alemancenter/fiber-api/pkg/logger"

	_ "github.com/alemancenter/fiber-api/docs" // Swagger docs

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/swagger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// @title Alemancenter API
// @version 2.0.0
// @description Backend API for Alemancenter Educational Platform.
// @termsOfService http://swagger.io/terms/
// @security FrontendKeyAuth

// @contact.name API Support
// @contact.email support@alemancenter.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host api.alemancenter.com
// @BasePath /api
// @schemes https http

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @securityDefinitions.apikey FrontendKeyAuth
// @in header
// @name X-Frontend-Key
// @description Frontend identifier key for public endpoints.

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	log := logger.Init(logger.Config{
		Level:      cfg.Log.Level,
		FilePath:   cfg.Log.Path,
		MaxSize:    cfg.Log.MaxSize,
		MaxBackups: cfg.Log.MaxBackups,
		MaxAge:     cfg.Log.MaxAge,
		Debug:      cfg.App.Debug || cfg.App.IsDevelopment(),
	})
	defer log.Sync()

	logger.Info("Starting Alemancenter Fiber API",
		zap.String("app", cfg.App.Name),
		zap.String("env", cfg.App.Env),
		zap.String("version", "2.0.0"),
	)

	// Initialize databases
	logger.Info("Connecting to databases...")
	dbManager := database.GetManager()
	healthResults := dbManager.HealthCheck()
	for country, ok := range healthResults {
		if !ok {
			logger.Error("Database connection failed", zap.String("country", country))
		}
	}

	// Auto-migrate: add any missing columns (safe — never drops existing data)
	migrateTargets := []interface{}{
		&models.Article{},
		&models.Conversation{},
		&models.Message{},
		&models.Setting{},
		&models.BlockedIP{},
		&models.TrustedIP{},
		&models.SecurityLog{},
		&models.VisitorTracking{},
		&models.VisitorSession{},
		&models.Comment{},
		&models.Permission{},
		&models.PolicyAuditRun{},
		&models.PolicyAuditFinding{},
		&models.ContentAIDecision{},
		&models.ContentAIIssue{},
		&models.ContentAISuggestion{},
		&models.ContentAIFixPreview{},
		&models.ContentAIApprovalLog{},
		&models.PushToken{},
		&models.EmailVerificationReminder{},
		&models.EmailBounceEvent{},
		&models.ContactMessage{},
	}
	seen := make(map[*gorm.DB]bool)
	for _, id := range []database.CountryID{database.CountryJordan, database.CountrySaudi, database.CountryEgypt, database.CountryPalestine} {
		db := dbManager.Get(id)
		if seen[db] {
			continue
		}
		seen[db] = true
		// Drop legacy incompatible FK constraints left by Laravel before migrating
		if db.Migrator().HasConstraint(&models.Article{}, "articles_grade_level_foreign") {
			db.Migrator().DropConstraint(&models.Article{}, "articles_grade_level_foreign")
		}
		if err := db.AutoMigrate(migrateTargets...); err != nil {
			logger.Warn("auto-migrate failed", zap.String("country", database.CountryCode(id)), zap.Error(err))
		}
		ensureContentAISchema(db, database.CountryCode(id))
	}
	ensurePermission("manage content audit")
	syncMailConfigFromDB()
	go services.BackfillVerifiedUserRoles()

	// Initialize Redis
	logger.Info("Connecting to Redis...")
	database.GetRedis()

	// Start background workers
	services.StartViewSyncWorker(1 * time.Minute)
	services.StartVisitorWorker(5 * time.Second)
	startContentAuditScheduler(cfg)
	services.StartEmailVerificationReminderScheduler(services.NewEmailVerificationReminderService(services.NewMailService(), services.NewJWTService()))

	// Periodically prune expired AI generation jobs from the in-memory store.
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			services.GetAIJobStore().Prune()
		}
	}()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:                 cfg.App.Name + " v2.0.0",
		ServerHeader:            "", // Don't expose server info
		StrictRouting:           false,
		CaseSensitive:           false,
		Immutable:               false,
		UnescapePath:            true,
		BodyLimit:               100 * 1024 * 1024, // 100MB
		ReadTimeout:             30 * time.Second,
		WriteTimeout:            120 * time.Second, // AI generation can take up to 90s
		IdleTimeout:             120 * time.Second,
		ReadBufferSize:          8192,
		WriteBufferSize:         8192,
		CompressedFileSuffix:    ".fiber.gz",
		ProxyHeader:             "X-Forwarded-For",
		EnableTrustedProxyCheck: true,
		TrustedProxies:          cfg.App.TrustedProxies,
		EnableIPValidation:      true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			message := "حدث خطأ داخلي في الخادم"

			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
				switch code {
				case fiber.StatusNotFound:
					message = "المسار المطلوب غير موجود"
				case fiber.StatusMethodNotAllowed:
					message = "طريقة الطلب غير مدعومة"
				case fiber.StatusTooManyRequests:
					message = "تم تجاوز الحد المسموح للطلبات"
				}
			}

			fields := []zap.Field{
				zap.Int("status", code),
				zap.String("method", c.Method()),
				zap.String("path", c.Path()),
				zap.String("ip", c.IP()),
				zap.String("error", err.Error()),
			}

			// 408 is usually produced by fasthttp before routing when a client,
			// proxy, browser preconnect, or scanner opens a socket but does not
			// complete the request within ReadTimeout. It is not an application
			// failure, so avoid noisy ERROR stacktraces.
			switch {
			case code == fiber.StatusRequestTimeout:
				logger.Debug("client request timed out", fields...)
			case code >= fiber.StatusBadRequest && code < fiber.StatusInternalServerError:
				logger.Warn("client request error", fields...)
			default:
				logger.Error("unhandled server error", append(fields, zap.Error(err))...)
			}

			return c.Status(code).JSON(utils.APIResponse{
				Success: false,
				Message: message,
			})
		},
	})

	// Recover from panics
	app.Use(recover.New(recover.Config{
		EnableStackTrace: cfg.App.Debug,
		StackTraceHandler: func(c *fiber.Ctx, e interface{}) {
			logger.Error("panic recovered",
				zap.String("path", c.Path()),
				zap.Any("error", e),
			)
		},
	}))

	// Method Override (Laravel style _method for PUT/PATCH/DELETE in POST forms)
	app.Use(middleware.MethodOverride())

	// Serve static storage files (uploads, settings images, etc.)
	// Next.js rewrites /storage/:path* → backend /storage/:path*
	// STORAGE_PATH must match Laravel's storage/app/public layout so that
	// a file stored at {STORAGE_PATH}/files/foo.doc is served at /storage/files/foo.doc
	storageRoot := cfg.Storage.Path
	if storageRoot == "" {
		storageRoot = "./storage/app/public"
	}
	app.Static("/storage", storageRoot, fiber.Static{
		Compress:  true,
		ByteRange: true,
		Browse:    false,
		MaxAge:    31536000,
	})

	// Register all routes — returns the shared handler deps so the scheduler
	// and the HTTP handler use the exact same BounceIMAPReader instance.
	// TriggerNow() from the dashboard then reaches the running scheduler goroutine.
	deps := routes.Setup(app)
	services.StartBounceIMAPScheduler(deps.BounceReader)

	// Setup Swagger UI route
	app.Get("/swagger/*", swagger.HandlerDefault)

	// 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(utils.APIResponse{
			Success: false,
			Message: "المسار المطلوب غير موجود",
		})
	})

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)
	logger.Info("Server starting", zap.String("addr", addr))

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.Listen(addr); err != nil {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	logger.Info("Server is running",
		zap.String("addr", addr),
		zap.String("url", cfg.App.URL),
	)

	// Wait for shutdown signal
	<-quit
	logger.Info("Shutting down server...")

	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		logger.Error("error during shutdown", zap.Error(err))
	}

	logger.Info("Server stopped gracefully")
}

func startContentAuditScheduler(cfg *config.Config) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("CONTENT_AUDIT_SCHEDULER")))
	if value == "0" || value == "false" || value == "off" || value == "disabled" {
		logger.Info("content audit scheduler disabled")
		return
	}

	interval := 24 * time.Hour
	if raw := strings.TrimSpace(os.Getenv("CONTENT_AUDIT_INTERVAL_HOURS")); raw != "" {
		hours, err := strconv.Atoi(raw)
		if err == nil && hours > 0 {
			interval = time.Duration(hours) * time.Hour
		} else {
			logger.Warn("invalid CONTENT_AUDIT_INTERVAL_HOURS; using default", zap.String("value", raw))
		}
	}

	initialDelay := 15 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("CONTENT_AUDIT_INITIAL_DELAY_MINUTES")); raw != "" {
		minutes, err := strconv.Atoi(raw)
		if err == nil && minutes >= 0 {
			initialDelay = time.Duration(minutes) * time.Minute
		} else {
			logger.Warn("invalid CONTENT_AUDIT_INITIAL_DELAY_MINUTES; using default", zap.String("value", raw))
		}
	}

	auditRepo := repositories.NewContentAuditRepository()
	auditSvc := contentauditService.NewService(auditRepo, contentauditService.Options{Config: cfg})
	auditSvc.StartScheduler(interval, initialDelay)
	logger.Info("content audit scheduler started", zap.Duration("interval", interval), zap.Duration("initial_delay", initialDelay))
}

func ensureContentAISchema(db *gorm.DB, country string) {
	alters := []string{
		"ALTER TABLE content_ai_decisions MODIFY COLUMN country_code VARCHAR(20) NOT NULL DEFAULT 'jo'",
		"ALTER TABLE content_ai_fix_previews MODIFY COLUMN country_code VARCHAR(20) NOT NULL DEFAULT 'jo'",
		"ALTER TABLE content_ai_decisions MODIFY COLUMN report_json LONGTEXT",
		"ALTER TABLE content_ai_fix_previews MODIFY COLUMN original_content LONGTEXT",
		"ALTER TABLE content_ai_fix_previews MODIFY COLUMN fixed_content LONGTEXT",
		"UPDATE content_ai_decisions SET country_code = 'jo', content_id = CONCAT('jo:', SUBSTRING_INDEX(content_id, ':', -1)) WHERE country_code IN ('alhurani_jo', 'jordan', 'Jordan') OR country_code LIKE '%_jo'",
		"UPDATE content_ai_fix_previews SET country_code = 'jo', content_id = CONCAT('jo:', SUBSTRING_INDEX(content_id, ':', -1)) WHERE country_code IN ('alhurani_jo', 'jordan', 'Jordan') OR country_code LIKE '%_jo'",
	}
	for _, stmt := range alters {
		if err := db.Exec(stmt).Error; err != nil {
			logger.Warn("content AI schema normalization skipped", zap.String("country", country), zap.Error(err))
		}
	}

	type indexDef struct {
		name  string
		table string
		cols  string
	}
	indexes := []indexDef{
		{"idx_ai_decisions_lookup", "content_ai_decisions", "content_type, content_id, country_code"},
		{"idx_ai_decisions_created", "content_ai_decisions", "created_at"},
		{"idx_ai_fix_decision_status", "content_ai_fix_previews", "decision_id, status, created_at"},
		{"idx_policy_findings_run_risk", "policy_audit_findings", "run_id, risk"},
		{"idx_policy_findings_run_type", "policy_audit_findings", "run_id, content_type"},
		{"idx_visitors_tracking_created", "visitors_tracking", "created_at"},
		{"idx_visitors_tracking_url_created", "visitors_tracking", "url(191), created_at"},
	}
	for _, idx := range indexes {
		var count int64
		db.Raw(
			"SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
			idx.table, idx.name,
		).Scan(&count)
		if count > 0 {
			continue
		}
		if err := db.Exec(fmt.Sprintf("CREATE INDEX %s ON %s(%s)", idx.name, idx.table, idx.cols)).Error; err != nil {
			logger.Warn("content AI index creation failed", zap.String("country", country), zap.String("index", idx.name), zap.Error(err))
		}
	}
}

func ensurePermission(name string) {
	permission := models.Permission{Name: name, GuardName: "api"}
	if err := database.DB().Where("name = ?", name).FirstOrCreate(&permission).Error; err != nil {
		logger.Warn("failed to ensure permission", zap.String("permission", name), zap.Error(err))
	}
}

// syncMailConfigFromDB loads mail settings saved in the settings table and updates the
// in-memory config so the mail service uses DB values after a server restart.
func syncMailConfigFromDB() {
	svc := services.NewSettingService(repositories.NewSettingRepository())
	settings, err := svc.GetAll(context.Background(), database.CountryJordan)
	if err != nil {
		logger.Warn("could not sync mail config from DB", zap.Error(err))
		return
	}
	cur := config.Get().Mail
	updated := false
	if v, ok := settings["mail_host"]; ok && v != "" {
		cur.Host = v
		updated = true
	}
	if v, ok := settings["mail_port"]; ok && v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cur.Port = p
			updated = true
		}
	}
	if v, ok := settings["mail_username"]; ok && v != "" {
		cur.Username = v
		updated = true
	}
	if v, ok := settings["mail_password"]; ok && v != "" {
		cur.Password = v
		updated = true
	}
	if v, ok := settings["mail_encryption"]; ok && v != "" {
		cur.Encryption = v
		updated = true
	}
	if v, ok := settings["mail_from_address"]; ok && v != "" {
		cur.FromAddress = v
		updated = true
	}
	if v, ok := settings["mail_from_name"]; ok && v != "" {
		cur.FromName = v
		updated = true
	}
	if updated {
		config.UpdateMailConfig(cur)
		logger.Info("mail config synced from database settings")
	}

	// Sync bounce mailbox IMAP settings.
	bounceCur := config.Get().Mail.Bounce
	bounceUpdated := false
	if v, ok := settings["bounce_imap_host"]; ok && v != "" {
		bounceCur.Host = v
		bounceUpdated = true
	}
	if v, ok := settings["bounce_imap_port"]; ok && v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			bounceCur.Port = p
			bounceUpdated = true
		}
	}
	if v, ok := settings["bounce_imap_username"]; ok && v != "" {
		bounceCur.Username = v
		bounceUpdated = true
	}
	if v, ok := settings["bounce_imap_password"]; ok && v != "" {
		bounceCur.Password = v
		bounceUpdated = true
	}
	if v, ok := settings["bounce_imap_tls"]; ok && v != "" {
		bounceCur.TLS = v == "true" || v == "1"
		bounceUpdated = true
	}
	if v, ok := settings["bounce_processor_enabled"]; ok && v != "" {
		bounceCur.Enabled = v == "true" || v == "1"
		bounceUpdated = true
	}
	if v, ok := settings["mail_bounce_address"]; ok && v != "" {
		cur2 := config.Get().Mail
		cur2.BounceAddress = v
		config.UpdateMailConfig(cur2)
	}
	if bounceUpdated {
		config.UpdateBounceConfig(bounceCur)
		logger.Info("bounce IMAP config synced from database settings")
	}

	gCur := config.Get().Google
	gUpdated := false
	if v, ok := settings["google_client_id"]; ok && v != "" {
		gCur.ClientID = v
		gUpdated = true
	}
	if v, ok := settings["google_client_secret"]; ok && v != "" {
		gCur.ClientSecret = v
		gUpdated = true
	}
	if v, ok := settings["google_redirect_uri"]; ok && v != "" {
		gCur.RedirectURI = v
		gUpdated = true
	}
	if gUpdated {
		config.UpdateGoogleConfig(gCur)
		logger.Info("google oauth config synced from database settings")
	}
}
