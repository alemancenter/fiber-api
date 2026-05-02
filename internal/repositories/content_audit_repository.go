package repositories

import (
	"context"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"gorm.io/gorm"
)

type ContentAuditRepository interface {
	CreateRun(ctx context.Context, run *models.PolicyAuditRun) error
	UpdateRun(ctx context.Context, run *models.PolicyAuditRun) error
	GetRun(ctx context.Context, id uint64) (*models.PolicyAuditRun, error)
	ListRuns(ctx context.Context, limit, offset int) ([]models.PolicyAuditRun, int64, error)
	ReplaceFindings(ctx context.Context, runID uint, findings []models.PolicyAuditFinding) error
	ListFindings(ctx context.Context, runID uint64, risk, contentType, search string, limit, offset int) ([]models.PolicyAuditFinding, int64, error)
	AllFindings(ctx context.Context, runID uint64) ([]models.PolicyAuditFinding, error)
}

type contentAuditRepository struct{}

func NewContentAuditRepository() ContentAuditRepository {
	return &contentAuditRepository{}
}

func (r *contentAuditRepository) CreateRun(ctx context.Context, run *models.PolicyAuditRun) error {
	return database.DB().WithContext(ctx).Create(run).Error
}

func (r *contentAuditRepository) UpdateRun(ctx context.Context, run *models.PolicyAuditRun) error {
	return database.DB().WithContext(ctx).Save(run).Error
}

func (r *contentAuditRepository) GetRun(ctx context.Context, id uint64) (*models.PolicyAuditRun, error) {
	var run models.PolicyAuditRun
	if err := database.DB().WithContext(ctx).First(&run, id).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *contentAuditRepository) ListRuns(ctx context.Context, limit, offset int) ([]models.PolicyAuditRun, int64, error) {
	var runs []models.PolicyAuditRun
	var total int64
	query := database.DB().WithContext(ctx).Model(&models.PolicyAuditRun{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("started_at DESC").Limit(limit).Offset(offset).Find(&runs).Error; err != nil {
		return nil, 0, err
	}
	return runs, total, nil
}

func (r *contentAuditRepository) ReplaceFindings(ctx context.Context, runID uint, findings []models.PolicyAuditFinding) error {
	return database.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("run_id = ?", runID).Delete(&models.PolicyAuditFinding{}).Error; err != nil {
			return err
		}
		if len(findings) == 0 {
			return nil
		}
		return tx.CreateInBatches(findings, 500).Error
	})
}

func (r *contentAuditRepository) ListFindings(ctx context.Context, runID uint64, risk, contentType, search string, limit, offset int) ([]models.PolicyAuditFinding, int64, error) {
	var findings []models.PolicyAuditFinding
	var total int64
	query := database.DB().WithContext(ctx).Model(&models.PolicyAuditFinding{}).Where("run_id = ?", runID)

	if risk != "" {
		query = query.Where("risk = ?", risk)
	}
	if contentType != "" {
		query = query.Where("content_type = ?", contentType)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("(content_id LIKE ? OR title LIKE ? OR reason LIKE ? OR url LIKE ?)", like, like, like, like)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("risk ASC, content_type ASC, content_id ASC").Limit(limit).Offset(offset).Find(&findings).Error; err != nil {
		return nil, 0, err
	}
	return findings, total, nil
}

func (r *contentAuditRepository) AllFindings(ctx context.Context, runID uint64) ([]models.PolicyAuditFinding, error) {
	var findings []models.PolicyAuditFinding
	err := database.DB().WithContext(ctx).
		Where("run_id = ?", runID).
		Order("risk ASC, content_type ASC, content_id ASC").
		Find(&findings).Error
	return findings, err
}
