package services

import (
	"time"

	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/alemancenter/fiber-api/pkg/logger"
	"go.uber.org/zap"
)

type SecurityService interface {
	LogEvent(ip, eventType, description, severity string, opts ...SecurityLogOption)
	BlockIP(ip, reason string, blockedBy *uint) error
	UnblockIP(ip string) error
	TrustIP(ip, note string, addedBy *uint) error
	UntrustIP(ip string) error
	IsBlocked(ip string) bool
	IsTrusted(ip string) bool

	GetStats() (totalLogs, criticalLogs, resolvedLogs, blockedIPs, trustedIPs int64, err error)
	GetLogs(severity, eventType, ip, resolved string, limit, offset int) ([]models.SecurityLog, int64, error)
	ResolveLog(id uint64) error
	DeleteLog(id uint64) error
	DeleteAllLogs() error
	GetOverviewStats(last24h, last7d time.Time) (last24hCount, last7dCount, totalAttacks int64, err error)
	GetTopAttackingIPs(limit int) ([]struct {
		IPAddress string `json:"ip_address"`
		Count     int64  `json:"count"`
	}, error)
	GetIPLogs(ip string, limit int) ([]models.SecurityLog, int64, error)
	GetBlockedIPs(limit, offset int) ([]models.BlockedIP, int64, error)
	GetTrustedIPs(limit, offset int) ([]models.TrustedIP, int64, error)
	GetAnalyticsBySeverity() ([]struct {
		Severity string `json:"severity"`
		Count    int64  `json:"count"`
	}, error)
	GetAnalyticsByEventType(limit int) ([]struct {
		EventType string `json:"event_type"`
		Count     int64  `json:"count"`
	}, error)
	GetTopRoutes(limit int) ([]struct {
		Route string `json:"route"`
		Count int64  `json:"count"`
	}, error)
	GetGeoDistribution(limit int) ([]struct {
		CountryCode string `json:"country_code"`
		Count       int64  `json:"count"`
	}, error)
}

// securityService handles security logging and IP management
type securityService struct {
	repo repositories.SecurityRepository
}

// NewSecurityService creates a new SecurityService
func NewSecurityService(repo repositories.SecurityRepository) SecurityService {
	return &securityService{repo: repo}
}

// LogEvent records a security event
func (s *securityService) LogEvent(ip, eventType, description, severity string, opts ...SecurityLogOption) {
	log := &models.SecurityLog{
		IPAddress:   ip,
		EventType:   eventType,
		Description: description,
		Severity:    severity,
	}

	for _, opt := range opts {
		opt(log)
	}

	go func() {
		if err := s.repo.CreateSecurityLog(log); err != nil {
			logger.Error("failed to create security log", zap.Error(err))
		}
	}()
}

// SecurityLogOption is a functional option for SecurityLog
type SecurityLogOption func(*models.SecurityLog)

// WithUserAgent sets the user agent
func WithUserAgent(ua string) SecurityLogOption {
	return func(l *models.SecurityLog) { l.UserAgent = &ua }
}

// WithRoute sets the route
func WithRoute(route string) SecurityLogOption {
	return func(l *models.SecurityLog) { l.Route = &route }
}

// WithMethod sets the HTTP method
func WithMethod(method string) SecurityLogOption {
	return func(l *models.SecurityLog) { l.Method = &method }
}

// WithRiskScore sets the risk score
func WithRiskScore(score int) SecurityLogOption {
	return func(l *models.SecurityLog) { l.RiskScore = score }
}

// WithUserID sets the user ID
func WithUserID(userID uint) SecurityLogOption {
	return func(l *models.SecurityLog) { l.UserID = &userID }
}

// WithAttackType sets the attack type
func WithAttackType(t string) SecurityLogOption {
	return func(l *models.SecurityLog) { l.AttackType = &t }
}

// BlockIP adds an IP to the blocked list
func (s *securityService) BlockIP(ip, reason string, blockedBy *uint) error {
	blocked := models.BlockedIP{
		IPAddress:   ip,
		IsAutoBlock: blockedBy == nil,
		BlockedBy:   blockedBy,
	}
	if reason != "" {
		blocked.Reason = &reason
	}
	return s.repo.BlockIP(ip, blocked)
}

// UnblockIP removes an IP from the blocked list
func (s *securityService) UnblockIP(ip string) error {
	return s.repo.UnblockIP(ip)
}

// TrustIP adds an IP to the trusted list
func (s *securityService) TrustIP(ip, note string, addedBy *uint) error {
	trusted := models.TrustedIP{
		IPAddress: ip,
		AddedBy:   addedBy,
	}
	if note != "" {
		trusted.Note = &note
	}
	return s.repo.TrustIP(ip, trusted)
}

// UntrustIP removes an IP from the trusted list
func (s *securityService) UntrustIP(ip string) error {
	return s.repo.UntrustIP(ip)
}

// IsBlocked checks if an IP is blocked
func (s *securityService) IsBlocked(ip string) bool {
	blocked, err := s.repo.GetBlockedIP(ip)
	if err != nil {
		return false
	}
	return !blocked.IsExpired()
}

// IsTrusted checks if an IP is trusted
func (s *securityService) IsTrusted(ip string) bool {
	_, err := s.repo.GetTrustedIP(ip)
	return err == nil
}

func (s *securityService) GetStats() (totalLogs, criticalLogs, resolvedLogs, blockedIPs, trustedIPs int64, err error) {
	return s.repo.GetStats()
}

func (s *securityService) GetLogs(severity, eventType, ip, resolved string, limit, offset int) ([]models.SecurityLog, int64, error) {
	return s.repo.GetLogs(severity, eventType, ip, resolved, limit, offset)
}

func (s *securityService) ResolveLog(id uint64) error {
	return s.repo.ResolveLog(id)
}

func (s *securityService) DeleteLog(id uint64) error {
	return s.repo.DeleteLog(id)
}

func (s *securityService) DeleteAllLogs() error {
	return s.repo.DeleteAllLogs()
}

func (s *securityService) GetOverviewStats(last24h, last7d time.Time) (last24hCount, last7dCount, totalAttacks int64, err error) {
	return s.repo.GetOverviewStats(last24h, last7d)
}

func (s *securityService) GetTopAttackingIPs(limit int) ([]struct {
	IPAddress string `json:"ip_address"`
	Count     int64  `json:"count"`
}, error) {
	return s.repo.GetTopAttackingIPs(limit)
}

func (s *securityService) GetIPLogs(ip string, limit int) ([]models.SecurityLog, int64, error) {
	return s.repo.GetIPLogs(ip, limit)
}

func (s *securityService) GetBlockedIPs(limit, offset int) ([]models.BlockedIP, int64, error) {
	return s.repo.GetBlockedIPs(limit, offset)
}

func (s *securityService) GetTrustedIPs(limit, offset int) ([]models.TrustedIP, int64, error) {
	return s.repo.GetTrustedIPs(limit, offset)
}

func (s *securityService) GetAnalyticsBySeverity() ([]struct {
	Severity string `json:"severity"`
	Count    int64  `json:"count"`
}, error) {
	return s.repo.GetAnalyticsBySeverity()
}

func (s *securityService) GetAnalyticsByEventType(limit int) ([]struct {
	EventType string `json:"event_type"`
	Count     int64  `json:"count"`
}, error) {
	return s.repo.GetAnalyticsByEventType(limit)
}

func (s *securityService) GetTopRoutes(limit int) ([]struct {
	Route string `json:"route"`
	Count int64  `json:"count"`
}, error) {
	return s.repo.GetTopRoutes(limit)
}

func (s *securityService) GetGeoDistribution(limit int) ([]struct {
	CountryCode string `json:"country_code"`
	Count       int64  `json:"count"`
}, error) {
	return s.repo.GetGeoDistribution(limit)
}

// LogActivity records a user activity
func LogActivity(description string, subjectType string, subjectID uint, causerID uint) {
	log := &models.ActivityLog{
		Description: description,
		SubjectType: &subjectType,
		SubjectID:   &subjectID,
		CauserType:  strPtr("App\\Models\\User"),
		CauserID:    &causerID,
	}

	go func() {
		// Use repository for loose coupling
		repo := repositories.NewSecurityRepository()
		if err := repo.CreateActivityLog(log); err != nil {
			logger.Error("failed to create activity log", zap.Error(err))
		}
	}()
}

func strPtr(s string) *string {
	return &s
}
