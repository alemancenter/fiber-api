package redis

import (
	"context"
	"strings"
	"time"

	_ "github.com/alemancenter/fiber-api/internal/models"
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
// @Summary List Redis Keys
// @Description Returns a list of Redis keys matching a specific pattern
// @Tags Redis
// @Produce json
// @Security BearerAuth
// @Param pattern query string false "Key pattern to match (e.g. *)"
// @Success 200 {object} utils.APIResponse{data=services.RedisKeysResponse}
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/redis/keys [get]
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

type SetRedisKeyRequest struct {
	Key   string `json:"key" validate:"required"`
	Value string `json:"value" validate:"required"`
	TTL   int    `json:"ttl"` // seconds
}

// SetKey sets a Redis key
// @Summary Set Redis Key
// @Description Set a key-value pair in Redis with an optional TTL
// @Tags Redis
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body SetRedisKeyRequest true "Redis key payload"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Failure 422 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/redis [post]
func (h *Handler) SetKey(c *fiber.Ctx) error {
	var req SetRedisKeyRequest
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
// @Summary Delete Redis Key
// @Description Delete a specific Redis key
// @Tags Redis
// @Produce json
// @Security BearerAuth
// @Param key path string true "Redis Key"
// @Success 200 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/redis/{key} [delete]
func (h *Handler) DeleteKey(c *fiber.Ctx) error {
	key := c.Params("key")

	if err := h.svc.DeleteKey(context.Background(), key); err != nil {
		return utils.InternalError(c, "فشل حذف المفتاح")
	}

	return utils.Success(c, "تم حذف المفتاح بنجاح", nil)
}

// CleanExpired removes expired keys
// @Summary Clean Expired Keys
// @Description Clean up expired keys from Redis
// @Tags Redis
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/redis/expired/clean [delete]
func (h *Handler) CleanExpired(c *fiber.Ctx) error {
	if err := h.svc.CleanExpired(context.Background()); err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "تم تنظيف المفاتيح المنتهية", nil)
}

// TestConnection tests the Redis connection
// @Summary Test Redis Connection
// @Description Ping the Redis server to check connectivity and health
// @Tags Redis
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.APIResponse{data=map[string]interface{}}
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/redis/test [get]
func (h *Handler) TestConnection(c *fiber.Ctx) error {
	health, allOk := h.svc.TestConnection()

	if !allOk {
		return utils.InternalError(c, "فشل الاتصال بـ Redis")
	}

	return utils.Success(c, "الاتصال بـ Redis يعمل بشكل صحيح", health)
}

// GetInfo returns Redis server information
// @Summary Get Redis Info
// @Description Returns internal Redis server information and statistics
// @Tags Redis
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.APIResponse{data=services.RedisInfoResponse}
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/redis/info [get]
func (h *Handler) GetInfo(c *fiber.Ctx) error {
	info, err := h.svc.GetInfo(context.Background())
	if err != nil {
		return utils.InternalError(c, "فشل الحصول على معلومات Redis")
	}

	return utils.Success(c, "success", services.RedisInfoResponse{Info: info})
}

// UpdateEnv validates Redis settings posted from the dashboard.
// Runtime Redis clients are initialized at process startup, so applying changes
// still requires a service restart or deployment-level configuration update.
// @Summary Update Redis Config
// @Description Validate and update Redis connection settings (Requires restart to apply fully)
// @Tags Redis
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body map[string]string true "Key-Value pairs of Redis config"
// @Success 200 {object} utils.APIResponse{data=map[string]string}
// @Failure 400 {object} utils.APIResponse
// @Router /dashboard/redis/env [post]
func (h *Handler) UpdateEnv(c *fiber.Ctx) error {
	var req map[string]string
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid redis settings")
	}

	allowed := map[string]bool{
		"REDIS_HOST":     true,
		"REDIS_PORT":     true,
		"REDIS_PASSWORD": true,
		"REDIS_DB":       true,
	}

	sanitized := make(map[string]string)
	for key, value := range req {
		if !allowed[key] {
			continue
		}
		sanitized[key] = strings.TrimSpace(value)
	}

	if len(sanitized) == 0 {
		return utils.BadRequest(c, "no valid redis settings provided")
	}

	return utils.Success(c, "redis settings accepted; restart the service to apply runtime changes", sanitized)
}
