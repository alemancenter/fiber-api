package services

import (
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
)

type DashboardStatsData struct {
	repositories.DashboardStats
	OnlineUsers int64 `json:"online_users"`
}

type DashboardHomeData struct {
	Stats            DashboardStatsData `json:"stats"`
	RecentActivities []ActivityOut      `json:"recentActivities"`
	RecentUsers      []models.User      `json:"recent_users"`
}

type DashboardService interface {
	GetHomeData(countryID database.CountryID) (*DashboardHomeData, error)
	ListActivities(logName, causerID string, offset, limit int) ([]models.ActivityLog, int64, error)
	CleanOldActivities() (int64, error)
}

type CleanActivitiesResponse struct {
	Deleted int64 `json:"deleted"`
}

type dashboardService struct {
	repo repositories.DashboardRepository
}

func NewDashboardService(repo repositories.DashboardRepository) DashboardService {
	return &dashboardService{repo: repo}
}

func (s *dashboardService) GetHomeData(countryID database.CountryID) (*DashboardHomeData, error) {
	stats, err := s.repo.GetStats(countryID)
	if err != nil {
		return nil, err
	}

	recentActivitiesRaw, err := s.repo.GetRecentActivity(10)
	if err != nil {
		return nil, err
	}

	activities := make([]ActivityOut, 0, len(recentActivitiesRaw))
	for _, a := range recentActivitiesRaw {
		atype := "article"
		if a.SubjectType != nil {
			switch *a.SubjectType {
			case "Post":
				atype = "news"
			case "Comment":
				atype = "comment"
			}
		}
		
		// Attempt to parse causer_name from properties or use a default if not available
		// The exact user name logic depends on how activity_log is populated
		// For now we'll use a placeholder or empty string if not explicitly stored in log
		userName := "مستخدم"
		
		activities = append(activities, ActivityOut{
			ID:        a.ID,
			Type:      atype,
			Title:     a.Description,
			User:      ActivityUser{Name: userName},
			CreatedAt: a.CreatedAt,
		})
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

	return &DashboardHomeData{
		Stats: DashboardStatsData{
			DashboardStats: *stats,
			OnlineUsers:    onlineCount,
		},
		RecentActivities: activities,
		RecentUsers:      recentUsers,
	}, nil
}

func (s *dashboardService) ListActivities(logName, causerID string, offset, limit int) ([]models.ActivityLog, int64, error) {
	return s.repo.ListActivities(logName, causerID, offset, limit)
}

func (s *dashboardService) CleanOldActivities() (int64, error) {
	cutoff := time.Now().AddDate(0, -3, 0) // 3 months ago
	return s.repo.CleanOldActivities(cutoff)
}
