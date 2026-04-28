package repositories

import (
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
)

type DashboardStats struct {
	Articles     int64 `json:"articles"`
	Posts        int64 `json:"posts"`
	Users        int64 `json:"users"`
	Comments     int64 `json:"comments"`
	Files        int64 `json:"files"`
	BlockedIPs   int64 `json:"blocked_ips"`
	SecurityLogs int64 `json:"security_logs"`
}

type DashboardRepository interface {
	GetStats(countryID database.CountryID) (*DashboardStats, error)
	GetRecentActivity(limit int) ([]models.ActivityLog, error)
	GetRecentUsers(limit int) ([]models.User, error)
	GetOnlineUsersCount(since time.Time) (int64, error)

	ListActivities(logName, causerID string, offset, limit int) ([]models.ActivityLog, int64, error)
	CleanOldActivities(cutoff time.Time) (int64, error)
}

type dashboardRepository struct{}

func NewDashboardRepository() DashboardRepository {
	return &dashboardRepository{}
}

func (r *dashboardRepository) GetStats(countryID database.CountryID) (*DashboardStats, error) {
	db := database.DBForCountry(countryID)
	mainDB := database.DB()

	var articles, posts, users, comments, files, blockedIPs, securityLogs int64

	db.Model(&models.Article{}).Count(&articles)
	db.Model(&models.Post{}).Count(&posts)
	mainDB.Model(&models.User{}).Count(&users)
	db.Model(&models.Comment{}).Count(&comments)
	db.Model(&models.File{}).Count(&files)
	mainDB.Model(&models.BlockedIP{}).Count(&blockedIPs)
	mainDB.Model(&models.SecurityLog{}).Where("severity IN ?", []string{"danger", "critical"}).Count(&securityLogs)

	stats := &DashboardStats{
		Articles:     articles,
		Posts:        posts,
		Users:        users,
		Comments:     comments,
		Files:        files,
		BlockedIPs:   blockedIPs,
		SecurityLogs: securityLogs,
	}

	return stats, nil
}

func (r *dashboardRepository) GetRecentActivity(limit int) ([]models.ActivityLog, error) {
	var recentActivity []models.ActivityLog
	err := database.DB().Order("created_at DESC").Limit(limit).Find(&recentActivity).Error
	return recentActivity, err
}

func (r *dashboardRepository) GetRecentUsers(limit int) ([]models.User, error) {
	var recentUsers []models.User
	err := database.DB().Select("id, name, email, created_at, status").
		Order("created_at DESC").Limit(limit).Find(&recentUsers).Error
	return recentUsers, err
}

func (r *dashboardRepository) GetOnlineUsersCount(since time.Time) (int64, error) {
	var onlineCount int64
	err := database.DB().Model(&models.User{}).Where("last_activity >= ?", since).Count(&onlineCount).Error
	return onlineCount, err
}

func (r *dashboardRepository) ListActivities(logName, causerID string, offset, limit int) ([]models.ActivityLog, int64, error) {
	var activities []models.ActivityLog
	var total int64

	query := database.DB().Model(&models.ActivityLog{})

	if logName != "" {
		query = query.Where("log_name = ?", logName)
	}
	if causerID != "" {
		query = query.Where("causer_id = ?", causerID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&activities).Error
	return activities, total, err
}

func (r *dashboardRepository) CleanOldActivities(cutoff time.Time) (int64, error) {
	result := database.DB().Where("created_at < ?", cutoff).Delete(&models.ActivityLog{})
	return result.RowsAffected, result.Error
}
