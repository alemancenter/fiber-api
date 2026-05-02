package models

import "time"

const (
	PolicyAuditStatusRunning   = "running"
	PolicyAuditStatusCompleted = "completed"
	PolicyAuditStatusFailed    = "failed"

	PolicyAuditTriggerManual    = "manual"
	PolicyAuditTriggerScheduled = "scheduled"
)

type PolicyAuditRun struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	Status            string     `gorm:"type:varchar(20);not null;index" json:"status"`
	TriggeredBy       string     `gorm:"type:varchar(30);not null;index" json:"triggered_by"`
	TriggeredByUserID *uint      `gorm:"index" json:"triggered_by_user_id,omitempty"`
	StartedAt         time.Time  `gorm:"not null;index" json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
	FindingsCount     int        `gorm:"default:0" json:"findings_count"`
	ErrorMessage      *string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`

	Findings []PolicyAuditFinding `gorm:"foreignKey:RunID" json:"findings,omitempty"`
}

func (PolicyAuditRun) TableName() string { return "policy_audit_runs" }

type PolicyAuditFinding struct {
	ID                uint      `gorm:"primaryKey" json:"finding_id"`
	RunID             uint      `gorm:"not null;index" json:"run_id"`
	ContentType       string    `gorm:"column:content_type;type:varchar(30);not null;index" json:"type"`
	ContentID         string    `gorm:"type:varchar(50);not null;index" json:"id"`
	Title             string    `gorm:"type:text" json:"title"`
	Risk              string    `gorm:"type:varchar(80);not null;index" json:"risk"`
	Reason            string    `gorm:"type:text" json:"reason"`
	URL               string    `gorm:"type:varchar(1000)" json:"url"`
	RecommendedAction string    `gorm:"type:text" json:"recommended_action"`
	CreatedAt         time.Time `json:"created_at"`
}

func (PolicyAuditFinding) TableName() string { return "policy_audit_findings" }
