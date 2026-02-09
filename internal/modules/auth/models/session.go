package models

import "time"

// Session represents an active user session
type Session struct {
	ID                  string     `gorm:"type:varchar(255);primaryKey" json:"id"`
	UserID              *string    `gorm:"type:char(26);index" json:"user_id,omitempty"`
	IPAddress           *string    `gorm:"type:varchar(45)" json:"ip_address,omitempty"`
	UserAgent           *string    `gorm:"type:text" json:"user_agent,omitempty"`
	Payload             string     `gorm:"type:longtext;not null" json:"-"`
	LastActivity        int        `gorm:"type:int;not null;index" json:"last_activity"`
	TwoFactorVerifiedAt *time.Time `gorm:"type:timestamp" json:"two_factor_verified_at,omitempty"`
}

// TableName returns the table name for the Session model
func (Session) TableName() string {
	return "sessions"
}
