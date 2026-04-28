package redis

import (
	"context"
	"time"

	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains Redis management route handlers
type Handler struct {
	svc services.RedisService
}

// New creates a new Redis Handler
func New(svc services.RedisService) *Handler {
	return &Handler{
		svc: svc,
	}
}

// ListKeys lists Redis keys matching a pattern
// GET /api/dashboard/redis/keys
func (h *Handler) ListKeys(c *fiber.Ctx) error {
	pattern := c.Query("pattern", "*")

	keys, err := h.svc.ListKeys(context.Background(), pattern)
	if err != nil {
		return utils.InternalError(c, "فشل الحصول على مفاتيح Redis")
	}

	return utils.Success(c, "success", services.RedisKeysResponse{
		Keys:  keys,
		Count: len(keys),
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

	ttl := time.Duration(req.TTL) * time.Second
	if req.TTL == 0 {
		ttl = 0 // No expiry
	}

	if err := h.svc.SetKey(context.Background(), req.Key, req.Value, ttl); err != nil {
		return utils.InternalError(c, "فشل تعيين المفتاح")
	}

	return utils.Success(c, "تم تعيين المفتاح بنجاح", nil)
}

// DeleteKey deletes a Redis key
// DELETE /api/dashboard/redis/:key
func (h *Handler) DeleteKey(c *fiber.Ctx) error {
	key := c.Params("key")

	if err := h.svc.DeleteKey(context.Background(), key); err != nil {
		return utils.InternalError(c, "فشل حذف المفتاح")
	}

	return utils.Success(c, "تم حذف المفتاح بنجاح", nil)
}

// CleanExpired removes expired keys
// DELETE /api/dashboard/redis/expired/clean
func (h *Handler) CleanExpired(c *fiber.Ctx) error {
	if err := h.svc.CleanExpired(context.Background()); err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "تم تنظيف المفاتيح المنتهية", nil)
}

// TestConnection tests the Redis connection
// GET /api/dashboard/redis/test
func (h *Handler) TestConnection(c *fiber.Ctx) error {
	health, allOk := h.svc.TestConnection()

	if !allOk {
		return utils.InternalError(c, "فشل الاتصال بـ Redis")
	}

	return utils.Success(c, "الاتصال بـ Redis يعمل بشكل صحيح", health)
}

// GetInfo returns Redis server information
// GET /api/dashboard/redis/info
func (h *Handler) GetInfo(c *fiber.Ctx) error {
	info, err := h.svc.GetInfo(context.Background())
	if err != nil {
		return utils.InternalError(c, "فشل الحصول على معلومات Redis")
	}

	return utils.Success(c, "success", services.RedisInfoResponse{Info: info})
}
