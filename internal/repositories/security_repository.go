package repositories

import (
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
)

type TopAttackingIP struct {
	IPAddress string `json:"ip_address"`
	Count     int64  `json:"count"`
}

type AnalyticRow struct {
	Severity  string `json:"severity,omitempty"`
	EventType string `json:"event_type,omitempty"`
	Count     int64  `json:"count"`
}

type SecurityRepository interface {
	GetStats() (totalLogs, criticalLogs, resolvedLogs, blockedIPs, trustedIPs int64, err error)
	GetLogs(severity, eventType, ip, resolved string, limit, offset int) ([]models.SecurityLog, int64, error)
	ResolveLog(id uint64) error
	DeleteLog(id uint64) error
	DeleteAllLogs() error
	GetOverviewStats(last24h, last7d time.Time) (last24hCount, last7dCount, totalAttacks int64, err error)
	GetTopAttackingIPs(limit int) ([]TopAttackingIP, error)
	GetIPLogs(ip string, limit int) ([]models.SecurityLog, int64, error)
	GetBlockedIPs(limit, offset int) ([]models.BlockedIP, int64, error)
	GetTrustedIPs(limit, offset int) ([]models.TrustedIP, int64, error)
	GetAnalyticsBySeverity() ([]AnalyticRow, error)
	GetAnalyticsByEventType(limit int) ([]AnalyticRow, error)
	GetTopRoutes(limit int) ([]struct {
		Route string `json:"route"`
		Count int64  `json:"count"`
	}, error)
	GetGeoDistribution(limit int) ([]struct {
		CountryCode string `json:"country_code"`
		Count       int64  `json:"count"`
	}, error)
	CreateSecurityLog(log *models.SecurityLog) error
	BlockIP(ip string, blocked models.BlockedIP) error
	UnblockIP(ip string) error
	TrustIP(ip string, trusted models.TrustedIP) error
	UntrustIP(ip string) error
	GetBlockedIP(ip string) (*models.BlockedIP, error)
	GetTrustedIP(ip string) (*models.TrustedIP, error)
	CreateActivityLog(log *models.ActivityLog) error
}

type securityRepository struct{}

func NewSecurityRepository() SecurityRepository {
	return &securityRepository{}
}

func (r *securityRepository) GetStats() (totalLogs, criticalLogs, resolvedLogs, blockedIPs, trustedIPs int64, err error) {
	db := database.DB()
	db.Model(&models.SecurityLog{}).Count(&totalLogs)
	db.Model(&models.SecurityLog{}).Where("severity = ?", models.SeverityCritical).Count(&criticalLogs)
	db.Model(&models.SecurityLog{}).Where("is_resolved = ?", true).Count(&resolvedLogs)
	db.Model(&models.BlockedIP{}).Count(&blockedIPs)
	db.Model(&models.TrustedIP{}).Count(&trustedIPs)
	return
}

func (r *securityRepository) GetLogs(severity, eventType, ip, resolved string, limit, offset int) ([]models.SecurityLog, int64, error) {
	db := database.DB()
	var logs []models.SecurityLog
	var total int64

	query := db.Model(&models.SecurityLog{})

	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if ip != "" {
		query = query.Where("ip_address = ?", ip)
	}

	switch resolved {
	case "1":
		query = query.Where("is_resolved = ?", true)
	case "0":
		query = query.Where("is_resolved = ?", false)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

func (r *securityRepository) ResolveLog(id uint64) error {
	db := database.DB()
	var log models.SecurityLog
	if err := db.First(&log, id).Error; err != nil {
		return err
	}

	log.IsResolved = true
	now := time.Now()
	log.ResolvedAt = &now

	return db.Save(&log).Error
}

func (r *securityRepository) DeleteLog(id uint64) error {
	return database.DB().Delete(&models.SecurityLog{}, id).Error
}

func (r *securityRepository) DeleteAllLogs() error {
	return database.DB().Where("1 = 1").Delete(&models.SecurityLog{}).Error
}

func (r *securityRepository) GetOverviewStats(last24h, last7d time.Time) (last24hCount, last7dCount, totalAttacks int64, err error) {
	db := database.DB()
	db.Model(&models.SecurityLog{}).Where("created_at >= ?", last24h).Count(&last24hCount)
	db.Model(&models.SecurityLog{}).Where("created_at >= ?", last7d).Count(&last7dCount)
	db.Model(&models.SecurityLog{}).Where("attack_type IS NOT NULL").Count(&totalAttacks)
	return
}

func (r *securityRepository) GetTopAttackingIPs(limit int) ([]TopAttackingIP, error) {
	var topIPs []TopAttackingIP
	err := database.DB().Model(&models.SecurityLog{}).
		Select("ip_address, COUNT(*) as count").
		Group("ip_address").
		Order("count DESC").
		Limit(limit).
		Scan(&topIPs).Error
	return topIPs, err
}

func (r *securityRepository) GetIPLogs(ip string, limit int) ([]models.SecurityLog, int64, error) {
	db := database.DB()
	var logs []models.SecurityLog
	var count int64
	db.Model(&models.SecurityLog{}).Where("ip_address = ?", ip).Count(&count)
	err := db.Where("ip_address = ?", ip).Order("created_at DESC").Limit(limit).Find(&logs).Error
	return logs, count, err
}

func (r *securityRepository) GetBlockedIPs(limit, offset int) ([]models.BlockedIP, int64, error) {
	db := database.DB()
	var blocked []models.BlockedIP
	var total int64
	db.Model(&models.BlockedIP{}).Count(&total)
	err := db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&blocked).Error
	return blocked, total, err
}

func (r *securityRepository) GetTrustedIPs(limit, offset int) ([]models.TrustedIP, int64, error) {
	db := database.DB()
	var trusted []models.TrustedIP
	var total int64
	db.Model(&models.TrustedIP{}).Count(&total)
	err := db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&trusted).Error
	return trusted, total, err
}

func (r *securityRepository) GetAnalyticsBySeverity() ([]AnalyticRow, error) {
	var bySeverity []AnalyticRow
	err := database.DB().Model(&models.SecurityLog{}).
		Select("severity, COUNT(*) as count").
		Group("severity").
		Scan(&bySeverity).Error
	return bySeverity, err
}

func (r *securityRepository) GetAnalyticsByEventType(limit int) ([]AnalyticRow, error) {
	var byEventType []AnalyticRow
	err := database.DB().Model(&models.SecurityLog{}).
		Select("event_type, COUNT(*) as count").
		Group("event_type").
		Order("count DESC").
		Limit(limit).
		Scan(&byEventType).Error
	return byEventType, err
}

func (r *securityRepository) GetTopRoutes(limit int) ([]struct {
	Route string `json:"route"`
	Count int64  `json:"count"`
}, error) {
	var routes []struct {
		Route string `json:"route"`
		Count int64  `json:"count"`
	}
	err := database.DB().Model(&models.SecurityLog{}).
		Select("route, COUNT(*) as count").
		Where("route IS NOT NULL").
		Group("route").
		Order("count DESC").
		Limit(limit).
		Scan(&routes).Error
	return routes, err
}

func (r *securityRepository) GetGeoDistribution(limit int) ([]struct {
	CountryCode string `json:"country_code"`
	Count       int64  `json:"count"`
}, error) {
	var geo []struct {
		CountryCode string `json:"country_code"`
		Count       int64  `json:"count"`
	}
	err := database.DB().Model(&models.SecurityLog{}).
		Select("country_code, COUNT(*) as count").
		Where("country_code IS NOT NULL").
		Group("country_code").
		Order("count DESC").
		Limit(limit).
		Scan(&geo).Error
	return geo, err
}

func (r *securityRepository) CreateSecurityLog(log *models.SecurityLog) error {
	return database.DB().Create(log).Error
}

func (r *securityRepository) BlockIP(ip string, blocked models.BlockedIP) error {
	return database.DB().Where(models.BlockedIP{IPAddress: ip}).
		Assign(blocked).
		FirstOrCreate(&blocked).Error
}

func (r *securityRepository) UnblockIP(ip string) error {
	return database.DB().Where("ip_address = ?", ip).Delete(&models.BlockedIP{}).Error
}

func (r *securityRepository) TrustIP(ip string, trusted models.TrustedIP) error {
	return database.DB().Where(models.TrustedIP{IPAddress: ip}).
		Assign(trusted).
		FirstOrCreate(&trusted).Error
}

func (r *securityRepository) UntrustIP(ip string) error {
	return database.DB().Where("ip_address = ?", ip).Delete(&models.TrustedIP{}).Error
}

func (r *securityRepository) GetBlockedIP(ip string) (*models.BlockedIP, error) {
	var blocked models.BlockedIP
	err := database.DB().Where("ip_address = ?", ip).First(&blocked).Error
	return &blocked, err
}

func (r *securityRepository) GetTrustedIP(ip string) (*models.TrustedIP, error) {
	var trusted models.TrustedIP
	err := database.DB().Where("ip_address = ?", ip).First(&trusted).Error
	return &trusted, err
}

func (r *securityRepository) CreateActivityLog(log *models.ActivityLog) error {
	return database.DB().Create(log).Error
}
