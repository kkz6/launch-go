package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/platform/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// PlatformUpdate represents a platform-wide update that can be applied to servers
type PlatformUpdate struct {
	basemodels.BaseModel
	Key         string                 `gorm:"column:key;type:varchar(100);uniqueIndex;not null" json:"key"`
	Title       string                 `gorm:"type:varchar(255);not null" json:"title"`
	Description string                 `gorm:"type:text;not null" json:"description"`
	Severity    types.UpdateSeverity   `gorm:"type:varchar(50);not null;default:'info'" json:"severity"`
	ServerTypes dbtype.JSONStringSlice `gorm:"type:json" json:"server_types,omitempty"`

	// Relations
	ServerUpdates []ServerPlatformUpdate `gorm:"foreignKey:PlatformUpdateID;references:ID" json:"server_updates,omitempty"`
}

func (PlatformUpdate) TableName() string {
	return "platform_updates"
}

// ServerPlatformUpdate tracks the status of a platform update on a specific server
type ServerPlatformUpdate struct {
	basemodels.BaseModel
	ServerID         string                   `gorm:"column:server_id;type:char(26);not null" json:"server_id"`
	PlatformUpdateID string                   `gorm:"column:platform_update_id;type:char(26);not null" json:"platform_update_id"`
	Status           types.ServerUpdateStatus `gorm:"type:varchar(50);not null;default:'pending'" json:"status"`
	TaskID           *string                  `gorm:"column:task_id;type:char(26)" json:"task_id,omitempty"`
	ErrorMessage     *string                  `gorm:"column:error_message;type:text" json:"error_message,omitempty"`
	CompletedAt      *time.Time               `gorm:"column:completed_at;type:timestamp null" json:"completed_at,omitempty"`

	// Relations
	PlatformUpdate *PlatformUpdate `gorm:"foreignKey:PlatformUpdateID;references:ID" json:"platform_update,omitempty"`
}

func (ServerPlatformUpdate) TableName() string {
	return "server_platform_updates"
}

// PlatformUpdateDismissal tracks which users have dismissed the banner for a given update
type PlatformUpdateDismissal struct {
	ID               string     `gorm:"type:char(26);primaryKey" json:"id"`
	UserID           string     `gorm:"column:user_id;type:char(26);not null;uniqueIndex:idx_user_platform_update_dismissal" json:"user_id"`
	PlatformUpdateID string     `gorm:"column:platform_update_id;type:char(26);not null;uniqueIndex:idx_user_platform_update_dismissal" json:"platform_update_id"`
	CreatedAt        *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
}

func (PlatformUpdateDismissal) TableName() string {
	return "platform_update_dismissals"
}
