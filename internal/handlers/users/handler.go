package users

import (
	"strconv"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/services"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Handler contains user management route handlers
type Handler struct{}

// New creates a new users Handler
func New() *Handler { return &Handler{} }

// List returns a paginated list of users
// GET /api/dashboard/users
func (h *Handler) List(c *fiber.Ctx) error {
	db := database.DB()
	pag := utils.GetPagination(c)

	var users []models.User
	var total int64

	query := db.Model(&models.User{}).Preload("Roles")

	if search := c.Query("search"); search != "" {
		query = query.Where("name LIKE ? OR email LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if role := c.Query("role"); role != "" {
		query = query.Joins("JOIN model_has_roles ON model_has_roles.model_id = users.id").
			Joins("JOIN roles ON roles.id = model_has_roles.role_id").
			Where("roles.name = ?", role)
	}

	query.Count(&total)
	query.Order("created_at DESC").Limit(pag.PerPage).Offset(pag.Offset).Find(&users)

	return utils.Paginated(c, "success", users, pag.BuildMeta(total))
}

// Search searches users by name or email
// GET /api/dashboard/users/search
func (h *Handler) Search(c *fiber.Ctx) error {
	q := c.Query("q", "")
	if len(q) < 2 {
		return utils.Success(c, "success", []models.User{})
	}

	db := database.DB()
	var users []models.User
	db.Select("id, name, email, profile_photo_path").
		Where("name LIKE ? OR email LIKE ?", "%"+q+"%", "%"+q+"%").
		Limit(10).Find(&users)

	return utils.Success(c, "success", users)
}

// Show returns a single user
// GET /api/dashboard/users/:user
func (h *Handler) Show(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("user"), 10, 64)
	if err != nil {
		return utils.BadRequest(c, "معرف غير صحيح")
	}

	db := database.DB()
	var user models.User
	if err := db.Preload("Roles.Permissions").Preload("Permissions").First(&user, id).Error; err != nil {
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
	type CreateRequest struct {
		Name     string `json:"name" validate:"required,min=2,max=255"`
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=8"`
		Roles    []uint `json:"roles"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	if errs := utils.Validate(req); errs != nil {
		return utils.ValidationError(c, errs)
	}

	db := database.DB()

	var count int64
	db.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		return utils.ValidationError(c, map[string]string{"email": "البريد الإلكتروني مستخدم بالفعل"})
	}

	user := models.User{
		Name:   req.Name,
		Email:  req.Email,
		Status: "active",
	}
	if err := user.HashPassword(req.Password); err != nil {
		return utils.InternalError(c)
	}

	if err := db.Create(&user).Error; err != nil {
		return utils.InternalError(c, "فشل إنشاء المستخدم")
	}

	// Assign roles
	if len(req.Roles) > 0 {
		var roles []models.Role
		db.Where("id IN ?", req.Roles).Find(&roles)
		db.Model(&user).Association("Roles").Replace(roles)
	}

	callerUser, _ := c.Locals("user").(*models.User)
	if callerUser != nil {
		services.LogActivity("أنشأ مستخدم: "+user.Email, "User", user.ID, callerUser.ID)
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

	db := database.DB()
	var user models.User
	if err := db.First(&user, id).Error; err != nil {
		return utils.NotFound(c)
	}

	type UpdateRequest struct {
		Name     string `json:"name" validate:"omitempty,min=2,max=255"`
		Phone    string `json:"phone"`
		JobTitle string `json:"job_title"`
		Gender   string `json:"gender" validate:"omitempty,oneof=male female other"`
		Country  string `json:"country"`
		Status   string `json:"status" validate:"omitempty,oneof=active inactive banned"`
		Password string `json:"password" validate:"omitempty,min=8"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}
	if req.JobTitle != "" {
		updates["job_title"] = req.JobTitle
	}
	if req.Gender != "" {
		updates["gender"] = req.Gender
	}
	if req.Country != "" {
		updates["country"] = req.Country
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Password != "" {
		if err := user.HashPassword(req.Password); err == nil {
			updates["password"] = user.Password
		}
	}

	if err := db.Model(&user).Updates(updates).Error; err != nil {
		return utils.InternalError(c, "فشل تحديث المستخدم")
	}

	callerUser, _ := c.Locals("user").(*models.User)
	if callerUser != nil {
		services.LogActivity("حدّث مستخدم: "+user.Email, "User", user.ID, callerUser.ID)
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

	type RolesPermissionsRequest struct {
		Roles       []uint `json:"roles"`
		Permissions []uint `json:"permissions"`
	}

	var req RolesPermissionsRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "بيانات غير صحيحة")
	}

	db := database.DB()
	var user models.User
	if err := db.First(&user, id).Error; err != nil {
		return utils.NotFound(c)
	}

	if len(req.Roles) > 0 {
		var roles []models.Role
		db.Where("id IN ?", req.Roles).Find(&roles)
		db.Model(&user).Association("Roles").Replace(roles)
	} else {
		db.Model(&user).Association("Roles").Clear()
	}

	if len(req.Permissions) > 0 {
		var permissions []models.Permission
		db.Where("id IN ?", req.Permissions).Find(&permissions)
		db.Model(&user).Association("Permissions").Replace(permissions)
	} else {
		db.Model(&user).Association("Permissions").Clear()
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

	// Prevent self-deletion
	callerUser, _ := c.Locals("user").(*models.User)
	if callerUser != nil && callerUser.ID == uint(id) {
		return utils.BadRequest(c, "لا يمكنك حذف حسابك الخاص")
	}

	db := database.DB()
	var user models.User
	if err := db.First(&user, id).Error; err != nil {
		return utils.NotFound(c)
	}

	if err := db.Delete(&user).Error; err != nil {
		return utils.InternalError(c, "فشل حذف المستخدم")
	}

	if callerUser != nil {
		services.LogActivity("حذف مستخدم: "+user.Email, "User", user.ID, callerUser.ID)
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

	callerUser, _ := c.Locals("user").(*models.User)
	if callerUser != nil {
		// Remove caller from deletion list
		filteredIDs := make([]uint, 0)
		for _, id := range req.IDs {
			if id != callerUser.ID {
				filteredIDs = append(filteredIDs, id)
			}
		}
		req.IDs = filteredIDs
	}

	db := database.DB()
	db.Where("id IN ?", req.IDs).Delete(&models.User{})

	return utils.Success(c, "تم حذف المستخدمين المحددين", fiber.Map{"deleted": len(req.IDs)})
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

	db := database.DB()
	db.Model(&models.User{}).Where("id IN ?", req.IDs).Update("status", req.Status)

	return utils.Success(c, "تم تحديث حالة المستخدمين بنجاح", nil)
}
