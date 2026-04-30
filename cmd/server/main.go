package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alemancenter/fiber-api/internal/config"
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/middleware"
	"github.com/alemancenter/fiber-api/internal/routes"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/alemancenter/fiber-api/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"
)

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
		if ok {
			logger.Info("Database connected", zap.String("country", country))
		} else {
			logger.Error("Database connection failed", zap.String("country", country))
		}
	}

	// Initialize Redis
	logger.Info("Connecting to Redis...")
	database.GetRedis()

	// Start background workers
	services.StartViewSyncWorker(1 * time.Minute)

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
		WriteTimeout:            30 * time.Second,
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

			logger.Error("unhandled error",
				zap.Int("status", code),
				zap.String("path", c.Path()),
				zap.Error(err),
			)

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
	storageRoot := cfg.Storage.Path
	if storageRoot == "" {
		storageRoot = "./storage"
	}
	app.Static("/storage", storageRoot, fiber.Static{
		Compress:  true,
		ByteRange: true,
		Browse:    false,
		MaxAge:    86400,
	})

	// Register all routes
	routes.Setup(app)

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
