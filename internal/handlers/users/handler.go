package users

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Handler contains user management route handlers
type Handler struct {
	svc services.UserService
}

// New creates a new users Handler
func New(svc services.UserService) *Handler {
	return &Handler{svc: svc}
}

// List returns a paginated list of users
// GET /api/dashboard/users
func (h *Handler) List(c *fiber.Ctx) error {
	pag := utils.GetPagination(c)
	search := c.Query("search")
	status := c.Query("status")
	role := c.Query("role")

	users, total, err := h.svc.List(search, status, role, pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c)
	}

	return utils.Paginated(c, "success", users, pag.BuildMeta(total))
}

// Search searches users by name or email (autocomplete for messaging).
// Accepts ?search= or ?q= for compatibility.
// GET /api/dashboard/users/search
func (h *Handler) Search(c *fiber.Ctx) error {
	q := c.Query("search", c.Query("q", ""))
	users, err := h.svc.Search(q)
	if err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "success", users)
}

// Show returns a single user
// GET /api/dashboard/users/:user
func (h *Handler) Show(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("user"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	user, err := h.svc.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", user)
}

// Create creates a new user
// POST /api/dashboard/users
func (h *Handler) Create(c *fiber.Ctx) error {
	var req services.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	var callerID uint
	if callerUser, ok := c.Locals("user").(*models.User); ok {
		callerID = uint(callerUser.ID)
	}

	user, err := h.svc.Create(&req, callerID)
	if err != nil {
		if err.Error() == "البريد الإلكتروني مستخدم بالفعل" {
			return utils.ValidationError(c, map[string]string{"email": err.Error()})
		}
		return utils.InternalError(c, err.Error())
	}

	return utils.Created(c, "تم إنشاء المستخدم بنجاح", user)
}

// Update updates a user
// PUT /api/dashboard/users/:user
func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("user"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	var req services.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	var callerID uint
	if callerUser, ok := c.Locals("user").(*models.User); ok {
		callerID = uint(callerUser.ID)
	}

	user, err := h.svc.Update(id, &req, callerID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, "تم تحديث المستخدم بنجاح", user)
}

// UpdateRolesPermissions updates user roles and permissions
// PUT /api/dashboard/users/:user/roles-permissions
func (h *Handler) UpdateRolesPermissions(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("user"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	var req services.RolesPermissionsRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if err := h.svc.UpdateRolesPermissions(id, &req); err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, "تم تحديث الأدوار والصلاحيات بنجاح", nil)
}

// Delete deletes a user
// DELETE /api/dashboard/users/:user
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("user"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	var callerID uint
	if callerUser, ok := c.Locals("user").(*models.User); ok {
		callerID = uint(callerUser.ID)
	}

	if err := h.svc.Delete(id, callerID); err != nil {
		if err.Error() == "لا يمكنك حذف حسابك الخاص" {
			return utils.BadRequest(c, err.Error())
		}
		if err == gorm.ErrRecordNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, "تم حذف المستخدم بنجاح", nil)
}

// BulkDelete deletes multiple users
// POST /api/dashboard/users/bulk-delete
func (h *Handler) BulkDelete(c *fiber.Ctx) error {
	type BulkDeleteRequest struct {
		IDs []uint `json:"ids" validate:"required,min=1"`
	}

	var req BulkDeleteRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	var callerID uint
	if callerUser, ok := c.Locals("user").(*models.User); ok {
		callerID = uint(callerUser.ID)
	}

	deletedCount, err := h.svc.BulkDelete(req.IDs, callerID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, "تم حذف المستخدمين المحددين", fiber.Map{"deleted": deletedCount})
}

// UpdateStatus updates status for multiple users
// POST /api/dashboard/users/update-status
func (h *Handler) UpdateStatus(c *fiber.Ctx) error {
	type UpdateStatusRequest struct {
		IDs    []uint `json:"ids" validate:"required,min=1"`
		Status string `json:"status" validate:"required,oneof=active inactive banned"`
	}

	var req UpdateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	if err := h.svc.UpdateStatus(req.IDs, req.Status); err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, "تم تحديث حالة المستخدمين بنجاح", nil)
}
