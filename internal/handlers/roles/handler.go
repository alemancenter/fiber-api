package roles

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Handler contains roles and permissions route handlers
type Handler struct {
	svc services.RoleService
}

// New creates a new roles Handler
func New(svc services.RoleService) *Handler {
	return &Handler{
		svc: svc,
	}
}

// ListRoles returns all roles
// GET /api/dashboard/roles
func (h *Handler) ListRoles(c *fiber.Ctx) error {
	roles, err := h.svc.ListRoles()
	if err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "success", roles)
}

// GetRole returns a single role
// GET /api/dashboard/roles/:id
func (h *Handler) GetRole(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	role, err := h.svc.GetRole(id)
	if err != nil {
		return utils.NotFound(c)
	}

	return utils.Success(c, "success", role)
}

// CreateRole creates a new role
// POST /api/dashboard/roles
func (h *Handler) CreateRole(c *fiber.Ctx) error {
	type CreateRequest struct {
		Name        string `json:"name" validate:"required,min=2,max=125"`
		Permissions []uint `json:"permissions"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	role, err := h.svc.CreateRole(req.Name, req.Permissions)
	if err != nil {
		if err.Error() == "اسم الدور مستخدم بالفعل" {
			return utils.ValidationError(c, map[string]string{"name": err.Error()})
		}
		return utils.InternalError(c, "فشل إنشاء الدور")
	}

	return utils.Created(c, "تم إنشاء الدور بنجاح", role)
}

// UpdateRole updates a role
// PUT /api/dashboard/roles/:id
func (h *Handler) UpdateRole(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	type UpdateRequest struct {
		Name        string `json:"name" validate:"omitempty,min=2,max=125"`
		Permissions []uint `json:"permissions"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	role, err := h.svc.UpdateRole(id, req.Name, req.Permissions)
	if err != nil {
		return utils.NotFound(c)
	}

	return utils.Success(c, "تم تحديث الدور بنجاح", role)
}

// DeleteRole deletes a role
// DELETE /api/dashboard/roles/:id
func (h *Handler) DeleteRole(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	if err := h.svc.DeleteRole(id); err != nil {
		return utils.InternalError(c)
	}

	return utils.Success(c, "تم حذف الدور بنجاح", nil)
}

// ListPermissions returns all permissions
// GET /api/dashboard/permissions
func (h *Handler) ListPermissions(c *fiber.Ctx) error {
	permissions, err := h.svc.ListPermissions()
	if err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "success", permissions)
}

// CreatePermission creates a new permission
// POST /api/dashboard/permissions
func (h *Handler) CreatePermission(c *fiber.Ctx) error {
	type CreateRequest struct {
		Name string `json:"name" validate:"required,min=2,max=125"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	permission, err := h.svc.CreatePermission(req.Name)
	if err != nil {
		return utils.InternalError(c, "فشل إنشاء الصلاحية")
	}

	return utils.Created(c, "تم إنشاء الصلاحية بنجاح", permission)
}

// UpdatePermission updates a permission
// PUT /api/dashboard/permissions/:id
func (h *Handler) UpdatePermission(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	type UpdateRequest struct {
		Name string `json:"name" validate:"required,min=2,max=125"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if err := h.svc.UpdatePermission(id, req.Name); err != nil {
		return utils.InternalError(c, "فشل تحديث الصلاحية")
	}

	return utils.Success(c, "تم تحديث الصلاحية بنجاح", nil)
}

// DeletePermission deletes a permission
// DELETE /api/dashboard/permissions/:id
func (h *Handler) DeletePermission(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	if err := h.svc.DeletePermission(id); err != nil {
		return utils.InternalError(c, "فشل حذف الصلاحية")
	}

	return utils.Success(c, "تم حذف الصلاحية بنجاح", nil)
}