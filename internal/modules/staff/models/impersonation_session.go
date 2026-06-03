package models

import "time"

// ImpersonationSession is the audit record of one "spectate as user" session.
// Append-only: a row is written when staff start impersonating and ended_at is
// stamped when they exit. FKs are intentionally not enforced so an audit record
// survives deletion of the referenced user/team.
type ImpersonationSession struct {
	ID           string     `gorm:"type:char(26);primaryKey" json:"id"`
	StaffID      string     `gorm:"column:staff_id;type:char(26);not null;index" json:"staff_id"`
	TargetUserID string     `gorm:"column:target_user_id;type:char(26);not null;index" json:"target_user_id"`
	TeamID       *string    `gorm:"column:team_id;type:char(26)" json:"team_id,omitempty"`
	StartedAt    time.Time  `gorm:"column:started_at;type:timestamp;not null" json:"started_at"`
	EndedAt      *time.Time `gorm:"column:ended_at;type:timestamp" json:"ended_at,omitempty"`
	Reason       *string    `gorm:"column:reason;type:text" json:"reason,omitempty"`
}

func (ImpersonationSession) TableName() string { return "impersonation_sessions" }
