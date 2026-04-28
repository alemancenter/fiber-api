package services

import (
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
)

type DashboardService interface {
	GetHomeData(countryID database.CountryID) (map[string]interface{}, error)
	ListActivities(logName, causerID string, offset, limit int) ([]models.ActivityLog, int64, error)
	CleanOldActivities() (int64, error)
}

type dashboardService struct {
	repo repositories.DashboardRepository
}

func NewDashboardService(repo repositories.DashboardRepository) DashboardService {
	return &dashboardService{repo: repo}
}

func (s *dashboardService) GetHomeData(countryID database.CountryID) (map[string]interface{}, error) {
	stats, err := s.repo.GetStats(countryID)
	if err != nil {
		return nil, err
	}

	recentActivity, err := s.repo.GetRecentActivity(10)
	if err != nil {
		return nil, err
	}

	recentUsers, err := s.repo.GetRecentUsers(5)
	if err != nil {
		return nil, err
	}

	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	onlineCount, err := s.repo.GetOnlineUsersCount(fiveMinAgo)
	if err != nil {
		return nil, err
	}

	stats["online_users"] = onlineCount

	return map[string]interface{}{
		"stats":           stats,
		"recent_activity": recentActivity,
		"recent_users":    recentUsers,
	}, nil
}

func (s *dashboardService) ListActivities(logName, causerID string, offset, limit int) ([]models.ActivityLog, int64, error) {
	return s.repo.ListActivities(logName, causerID, offset, limit)
}

func (s *dashboardService) CleanOldActivities() (int64, error) {
	cutoff := time.Now().AddDate(0, -3, 0) // 3 months ago
	return s.repo.CleanOldActivities(cutoff)
}