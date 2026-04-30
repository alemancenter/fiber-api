package users

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
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
// @Summary List Users
// @Description Returns a paginated list of all users
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param search query string false "Search query"
// @Param status query string false "Filter by status"
// @Param role query string false "Filter by role"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} utils.APIResponse{data=[]services.UserResponse}
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/users [get]
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
// @Summary Search Users
// @Description Search users by name or email (used for autocomplete)
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param q query string false "Search query"
// @Success 200 {object} utils.APIResponse{data=[]services.UserResponse}
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/users/search [get]
func (h *Handler) Search(c *fiber.Ctx) error {
	q := c.Query("search", c.Query("q", ""))
	users, err := h.svc.Search(q)
	if err != nil {
		return utils.InternalError(c)
	}
	return utils.Success(c, "success", users)
}

// Show returns a single user
// @Summary Get User
// @Description Get user details by ID
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param user path int true "User ID"
// @Success 200 {object} utils.APIResponse{data=services.UserResponse}
// @Failure 400 {object} utils.APIResponse
// @Failure 404 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/users/{user} [get]
func (h *Handler) Show(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("user"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	user, err := h.svc.GetByID(id)
	if err != nil {
		if err == services.ErrNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c)
	}

	return utils.Success(c, "success", user)
}

// Create creates a new user
// @Summary Create User
// @Description Create a new user from the dashboard
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body services.CreateUserRequest true "User data"
// @Success 201 {object} utils.APIResponse{data=services.UserResponse}
// @Failure 400 {object} utils.APIResponse
// @Failure 422 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/users [post]
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
// @Summary Update User
// @Description Update an existing user from the dashboard
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user path int true "User ID"
// @Param request body services.UpdateUserRequest true "User data"
// @Success 200 {object} utils.APIResponse{data=services.UserResponse}
// @Failure 400 {object} utils.APIResponse
// @Failure 422 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/users/{user} [put]
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
// @Summary Update User Roles
// @Description Update the roles and direct permissions for a specific user
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user path int true "User ID"
// @Param request body services.RolesPermissionsRequest true "Roles and permissions data"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Failure 404 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/users/{user}/roles-permissions [put]
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
		if err == services.ErrNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, "تم تحديث الأدوار والصلاحيات بنجاح", nil)
}

// Delete deletes a user
// @Summary Delete User
// @Description Delete a user by ID
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param user path int true "User ID"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Failure 404 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/users/{user} [delete]
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
		if err == services.ErrNotFound {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, "تم حذف المستخدم بنجاح", nil)
}

type BulkDeleteRequest struct {
	IDs []uint `json:"ids" validate:"required,min=1"`
}

// BulkDelete deletes multiple users
// @Summary Bulk Delete Users
// @Description Delete multiple users by providing a list of IDs
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body BulkDeleteRequest true "List of user IDs to delete"
// @Success 200 {object} utils.APIResponse{data=services.BulkDeleteUsersResponse}
// @Failure 400 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/users/bulk-delete [post]
func (h *Handler) BulkDelete(c *fiber.Ctx) error {
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

	return utils.Success(c, "تم حذف المستخدمين المحددين", services.BulkDeleteUsersResponse{Deleted: deletedCount})
}

type UpdateStatusRequest struct {
	IDs    []uint `json:"ids" validate:"required,min=1"`
	Status string `json:"status" validate:"required,oneof=active inactive banned"`
}

// UpdateStatus updates status for multiple users
// @Summary Bulk Update Users Status
// @Description Update the status (active, inactive, banned) for multiple users simultaneously
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateStatusRequest true "List of IDs and new status"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Failure 422 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Router /dashboard/users/update-status [post]
func (h *Handler) UpdateStatus(c *fiber.Ctx) error {
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
