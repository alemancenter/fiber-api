package services

import (
	"errors"

	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
)

type UserService interface {
	List(search, status, role string, limit, offset int) ([]models.User, int64, error)
	Search(query string) ([]models.User, error)
	GetByID(id uint64) (*models.User, error)
	Create(req *CreateUserRequest, callerID uint) (*models.User, error)
	Update(id uint64, req *UpdateUserRequest, callerID uint) (*models.User, error)
	UpdateRolesPermissions(id uint64, req *RolesPermissionsRequest) error
	Delete(id uint64, callerID uint) error
	BulkDelete(ids []uint, callerID uint) (int, error)
	UpdateStatus(ids []uint, status string) error
}

type CreateUserRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=255"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Roles    []uint `json:"roles"`
}

type UpdateUserRequest struct {
	Name     string `json:"name" validate:"omitempty,min=2,max=255"`
	Phone    string `json:"phone"`
	JobTitle string `json:"job_title"`
	Gender   string `json:"gender" validate:"omitempty,oneof=male female other"`
	Country  string `json:"country"`
	Status   string `json:"status" validate:"omitempty,oneof=active inactive banned"`
	Password string `json:"password" validate:"omitempty,min=8"`
}

type RolesPermissionsRequest struct {
	Roles       []uint `json:"roles"`
	Permissions []uint `json:"permissions"`
}

type userService struct {
	repo repositories.UserRepository
}

func NewUserService(repo repositories.UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) List(search, status, role string, limit, offset int) ([]models.User, int64, error) {
	return s.repo.List(search, status, role, limit, offset)
}

func (s *userService) Search(query string) ([]models.User, error) {
	if len(query) < 2 {
		return []models.User{}, nil
	}
	return s.repo.Search(query, 10)
}

func (s *userService) GetByID(id uint64) (*models.User, error) {
	return s.repo.FindByID(id)
}

func (s *userService) Create(req *CreateUserRequest, callerID uint) (*models.User, error) {
	count, err := s.repo.CountByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("البريد الإلكتروني مستخدم بالفعل")
	}

	user := &models.User{
		Name:   req.Name,
		Email:  req.Email,
		Status: "active",
	}
	if err := user.HashPassword(req.Password); err != nil {
		return nil, err
	}

	if err := s.repo.Create(user); err != nil {
		return nil, errors.New("فشل إنشاء المستخدم")
	}

	if len(req.Roles) > 0 {
		if err := AssignRoles(s.repo.GetDB(), user.ID, req.Roles); err != nil {
			return nil, errors.New("فشل تعيين الأدوار")
		}
	}

	if callerID > 0 {
		LogActivity("أنشأ مستخدم: "+user.Email, "User", user.ID, callerID)
	}

	return user, nil
}

func (s *userService) Update(id uint64, req *UpdateUserRequest, callerID uint) (*models.User, error) {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
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

	if err := s.repo.Update(user, updates); err != nil {
		return nil, errors.New("فشل تحديث المستخدم")
	}

	if callerID > 0 {
		LogActivity("قام بتحديث مستخدم: "+user.Email, "User", user.ID, callerID)
	}

	return user, nil
}

func (s *userService) UpdateRolesPermissions(id uint64, req *RolesPermissionsRequest) error {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	db := s.repo.GetDB()

	if err := AssignRoles(db, user.ID, req.Roles); err != nil {
		return errors.New("فشل تحديث الأدوار")
	}
	if err := AssignPermissions(db, user.ID, req.Permissions); err != nil {
		return errors.New("فشل تحديث الصلاحيات")
	}

	InvalidateUserCache(user.ID)

	return nil
}

func (s *userService) Delete(id uint64, callerID uint) error {
	if callerID > 0 && callerID == uint(id) {
		return errors.New("لا يمكنك حذف حسابك الخاص")
	}

	user, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(user); err != nil {
		return errors.New("فشل حذف المستخدم")
	}

	if callerID > 0 {
		LogActivity("حذف مستخدم: "+user.Email, "User", user.ID, callerID)
	}

	return nil
}

func (s *userService) BulkDelete(ids []uint, callerID uint) (int, error) {
	filteredIDs := make([]uint, 0)
	for _, id := range ids {
		if callerID == 0 || id != callerID {
			filteredIDs = append(filteredIDs, id)
		}
	}

	if len(filteredIDs) == 0 {
		return 0, nil
	}

	if err := s.repo.BulkDelete(filteredIDs); err != nil {
		return 0, errors.New("فشل حذف المستخدمين المحددين")
	}

	return len(filteredIDs), nil
}

func (s *userService) UpdateStatus(ids []uint, status string) error {
	if err := s.repo.UpdateStatus(ids, status); err != nil {
		return errors.New("فشل تحديث حالة المستخدمين")
	}
	return nil
}
