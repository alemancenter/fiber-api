package services

import (
	"sync"
	"time"

	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/google/uuid"
)

type NotificationService interface {
	List(userID uint, unreadOnly bool, offset, limit int) ([]models.Notification, int64, error)
	GetLatest(userID uint, limit int) ([]models.Notification, int64, error)
	MarkAsRead(id string, userID uint) error
	MarkAllRead(userID uint) error
	Create(reqType string, notifiableID uint, data string) (*models.Notification, error)
	Delete(id string, userID uint) error
	Prune(daysOld int) (int64, error)
	BulkAction(action string, ids []string, userID uint) error
}

type PruneNotificationsResponse struct {
	Deleted int64 `json:"deleted"`
}

type LatestNotificationsResponse struct {
	Notifications []models.Notification `json:"notifications"`
	UnreadCount   int64                 `json:"unread_count"`
}

type notificationService struct {
	repo repositories.NotificationRepository
}

func NewNotificationService(repo repositories.NotificationRepository) NotificationService {
	return &notificationService{repo: repo}
}

func (s *notificationService) List(userID uint, unreadOnly bool, offset, limit int) ([]models.Notification, int64, error) {
	return s.repo.List(userID, unreadOnly, offset, limit)
}

func (s *notificationService) GetLatest(userID uint, limit int) ([]models.Notification, int64, error) {
	var (
		wg            sync.WaitGroup
		notifications []models.Notification
		unreadCount   int64
		err1, err2    error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		notifications, err1 = s.repo.GetLatest(userID, limit)
	}()
	go func() {
		defer wg.Done()
		unreadCount, err2 = s.repo.GetUnreadCount(userID)
	}()
	wg.Wait()

	if err1 != nil {
		return nil, 0, err1
	}
	if err2 != nil {
		return nil, 0, err2
	}

	return notifications, unreadCount, nil
}

func (s *notificationService) MarkAsRead(id string, userID uint) error {
	return s.repo.MarkAsRead(id, userID)
}

func (s *notificationService) MarkAllRead(userID uint) error {
	return s.repo.MarkAllRead(userID)
}

func (s *notificationService) Create(reqType string, notifiableID uint, data string) (*models.Notification, error) {
	notification := &models.Notification{
		ID:             uuid.New().String(),
		Type:           reqType,
		NotifiableType: "App\\Models\\User",
		NotifiableID:   notifiableID,
		Data:           data,
	}

	err := s.repo.Create(notification)
	return notification, err
}

func (s *notificationService) Delete(id string, userID uint) error {
	return s.repo.Delete(id, userID)
}

func (s *notificationService) Prune(daysOld int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -daysOld)
	return s.repo.Prune(cutoff)
}

func (s *notificationService) BulkAction(action string, ids []string, userID uint) error {
	switch action {
	case "read":
		return s.repo.BulkMarkAsRead(ids, userID)
	case "delete":
		return s.repo.BulkDelete(ids, userID)
	default:
		return nil // Action validation is handled by handler
	}
}
