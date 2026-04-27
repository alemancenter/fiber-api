package services

import (
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/pkg/logger"
	"go.uber.org/zap"
)

// SecurityService handles security logging and IP management
type SecurityService struct{}

// NewSecurityService creates a new SecurityService
func NewSecurityService() *SecurityService {
	return &SecurityService{}
}

// LogEvent records a security event
func (s *SecurityService) LogEvent(ip, eventType, description, severity string, opts ...SecurityLogOption) {
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
		db := database.DB()
		if err := db.Create(log).Error; err != nil {
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
func (s *SecurityService) BlockIP(ip, reason string, blockedBy *uint) error {
	db := database.DB()
	blocked := models.BlockedIP{
		IPAddress:   ip,
		IsAutoBlock: blockedBy == nil,
		BlockedBy:   blockedBy,
	}
	if reason != "" {
		blocked.Reason = &reason
	}
	return db.Where(models.BlockedIP{IPAddress: ip}).
		Assign(blocked).
		FirstOrCreate(&blocked).Error
}

// UnblockIP removes an IP from the blocked list
func (s *SecurityService) UnblockIP(ip string) error {
	db := database.DB()
	return db.Where("ip_address = ?", ip).Delete(&models.BlockedIP{}).Error
}

// TrustIP adds an IP to the trusted list
func (s *SecurityService) TrustIP(ip, note string, addedBy *uint) error {
	db := database.DB()
	trusted := models.TrustedIP{
		IPAddress: ip,
		AddedBy:   addedBy,
	}
	if note != "" {
		trusted.Note = &note
	}
	return db.Where(models.TrustedIP{IPAddress: ip}).
		Assign(trusted).
		FirstOrCreate(&trusted).Error
}

// UntrustIP removes an IP from the trusted list
func (s *SecurityService) UntrustIP(ip string) error {
	db := database.DB()
	return db.Where("ip_address = ?", ip).Delete(&models.TrustedIP{}).Error
}

// IsBlocked checks if an IP is blocked
func (s *SecurityService) IsBlocked(ip string) bool {
	db := database.DB()
	var blocked models.BlockedIP
	if err := db.Where("ip_address = ?", ip).First(&blocked).Error; err != nil {
		return false
	}
	return !blocked.IsExpired()
}

// IsTrusted checks if an IP is trusted
func (s *SecurityService) IsTrusted(ip string) bool {
	db := database.DB()
	var trusted models.TrustedIP
	return db.Where("ip_address = ?", ip).First(&trusted).Error == nil
}

// LogActivity records a user activity
func LogActivity(description string, subjectType string, subjectID uint, causerID uint) {
	log := models.ActivityLog{
		Description: description,
		SubjectType: &subjectType,
		SubjectID:   &subjectID,
		CauserType:  strPtr("App\\Models\\User"),
		CauserID:    &causerID,
	}

	go func() {
		db := database.DB()
		if err := db.Create(&log).Error; err != nil {
			logger.Error("failed to create activity log", zap.Error(err))
		}
	}()
}

func strPtr(s string) *string { return &s }
