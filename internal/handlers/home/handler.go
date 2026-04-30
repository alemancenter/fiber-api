package home

import (
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc services.HomeService
}

func New(svc services.HomeService) *Handler {
	return &Handler{svc: svc}
}

// GetHome returns all the necessary data for the frontend home page.
// GET /api/home
func (h *Handler) GetHome(c *fiber.Ctx) error {
	var countryID database.CountryID
	if cid, ok := c.Locals("country_id").(database.CountryID); ok && cid != 0 {
		countryID = cid
	} else {
		countryHeader := c.Get("X-Country-Id")
		if countryHeader == "" {
			countryHeader = c.Get("X-Country")
		}
		if countryHeader == "" {
			countryHeader = c.Query("country", "1")
		}
		countryID = database.CountryIDFromHeader(countryHeader)
	}

	data, err := h.svc.GetHome(countryID)
	if err != nil {
		return utils.InternalError(c, "فشل جلب بيانات الصفحة الرئيسية")
	}

	// Tell the browser to cache this for 10 minutes
	c.Set("Cache-Control", "public, max-age=600")

	return utils.Success(c, "success", data)
}
