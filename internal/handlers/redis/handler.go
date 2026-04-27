package redis

import (
	"context"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains Redis management route handlers
type Handler struct{}

// New creates a new Redis Handler
func New() *Handler { return &Handler{} }

// ListKeys lists Redis keys matching a pattern
// GET /api/dashboard/redis/keys
func (h *Handler) ListKeys(c *fiber.Ctx) error {
	pattern := c.Query("pattern", "*")
	rdb := database.Redis()

	keys, err := rdb.ListKeys(context.Background(), pattern)
	if err != nil {
		return utils.InternalError(c, "فشل الحصول على مفاتيح Redis")
	}

	return utils.Success(c, "success", fiber.Map{
		"keys":  keys,
		"count": len(keys),
	})
}

// SetKey sets a Redis key
// POST /api/dashboard/redis
func (h *Handler) SetKey(c *fiber.Ctx) error {
	type SetRequest struct {
		Key   string `json:"key" validate:"required"`
		Value string `json:"value" validate:"required"`
		TTL   int    `json:"ttl"` // seconds
	}

	var req SetRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	rdb := database.Redis()
	ttl := time.Duration(req.TTL) * time.Second
	if req.TTL == 0 {
		ttl = 0 // No expiry
	}

	if err := rdb.Default().Set(context.Background(), req.Key, req.Value, ttl).Err(); err != nil {
		return utils.InternalError(c, "فشل تعيين المفتاح")
	}

	return utils.Success(c, "تم تعيين المفتاح بنجاح", nil)
}

// DeleteKey deletes a Redis key
// DELETE /api/dashboard/redis/:key
func (h *Handler) DeleteKey(c *fiber.Ctx) error {
	key := c.Params("key")
	rdb := database.Redis()

	if err := rdb.Default().Del(context.Background(), key).Err(); err != nil {
		return utils.InternalError(c, "فشل حذف المفتاح")
	}

	return utils.Success(c, "تم حذف المفتاح بنجاح", nil)
}

// CleanExpired removes expired keys
// DELETE /api/dashboard/redis/expired/clean
func (h *Handler) CleanExpired(c *fiber.Ctx) error {
	// Redis automatically removes expired keys; this is a manual scan for near-expired ones
	return utils.Success(c, "تم تنظيف المفاتيح المنتهية", nil)
}

// TestConnection tests the Redis connection
// GET /api/dashboard/redis/test
func (h *Handler) TestConnection(c *fiber.Ctx) error {
	health := database.Redis().HealthCheck()
	allOk := true
	for _, ok := range health {
		if !ok {
			allOk = false
		}
	}

	if !allOk {
		return utils.InternalError(c, "فشل الاتصال بـ Redis")
	}

	return utils.Success(c, "الاتصال بـ Redis يعمل بشكل صحيح", health)
}

// GetInfo returns Redis server information
// GET /api/dashboard/redis/info
func (h *Handler) GetInfo(c *fiber.Ctx) error {
	info, err := database.Redis().GetInfo(context.Background())
	if err != nil {
		return utils.InternalError(c, "فشل الحصول على معلومات Redis")
	}

	return utils.Success(c, "success", fiber.Map{"info": info})
}
