package models

import (
	"time"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Metric represents server performance metrics
// Note: Uses auto-increment ID instead of ULID for high-volume time-series data
type Metric struct {
	ID          uint64     `gorm:"type:bigint unsigned;primaryKey;autoIncrement" json:"id"`
	ServerID    string     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	Load        float64    `gorm:"type:decimal(5,2);not null" json:"load"`
	MemoryTotal float64    `gorm:"column:memory_total;type:decimal(15,0);not null" json:"memory_total"`
	MemoryUsed  float64    `gorm:"column:memory_used;type:decimal(15,0);not null" json:"memory_used"`
	MemoryFree  float64    `gorm:"column:memory_free;type:decimal(15,0);not null" json:"memory_free"`
	DiskTotal   float64    `gorm:"column:disk_total;type:decimal(15,0);not null" json:"disk_total"`
	DiskUsed    float64    `gorm:"column:disk_used;type:decimal(15,0);not null" json:"disk_used"`
	DiskFree    float64    `gorm:"column:disk_free;type:decimal(15,0);not null" json:"disk_free"`
	CreatedAt   *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt   *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}

func (Metric) TableName() string {
	return "metrics"
}

// MemoryUsagePercent returns the memory usage as a percentage
func (m *Metric) MemoryUsagePercent() float64 {
	if m.MemoryTotal == 0 {
		return 0
	}

	return (m.MemoryUsed / m.MemoryTotal) * 100
}

// DiskUsagePercent returns the disk usage as a percentage
func (m *Metric) DiskUsagePercent() float64 {
	if m.DiskTotal == 0 {
		return 0
	}

	return (m.DiskUsed / m.DiskTotal) * 100
}

// Script represents a reusable script template
type Script struct {
	basemodels.BaseModel
	TeamID  string `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	UserID  string `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	Name    string `gorm:"type:varchar(255);not null" json:"name"`
	Content string `gorm:"type:longtext;not null" json:"content"`
}

func (Script) TableName() string {
	return "scripts"
}

// ScriptExecution represents an execution of a script on a server
// Note: Uses auto-increment ID for high-volume execution records
type ScriptExecution struct {
	ID         uint64     `gorm:"type:bigint unsigned;primaryKey;autoIncrement" json:"id"`
	ScriptID   string     `gorm:"column:script_id;type:char(26);not null;index" json:"script_id"`
	ServerID   string     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	User       *string    `gorm:"type:varchar(255)" json:"user,omitempty"`
	FinishedAt *time.Time `gorm:"column:finished_at;type:timestamp null" json:"finished_at,omitempty"`
	CreatedAt  *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt  *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Script *Script `gorm:"foreignKey:ScriptID;references:ID" json:"script,omitempty"`
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}

func (ScriptExecution) TableName() string {
	return "script_executions"
}

// Monitor represents a server monitoring rule
// Note: Uses auto-increment ID
type Monitor struct {
	ID        uint64     `gorm:"type:bigint unsigned;primaryKey;autoIncrement" json:"id"`
	ServerID  string     `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	Type      string     `gorm:"type:varchar(255);not null" json:"type"`
	Operator  string     `gorm:"type:varchar(255);not null" json:"operator"`
	Threshold float64    `gorm:"type:decimal(8,2);not null" json:"threshold"`
	Duration  int        `gorm:"type:int;not null" json:"duration"`
	CreatedAt *time.Time `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt *time.Time `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Server               *Server               `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
	NotificationChannels []NotificationChannel `gorm:"many2many:monitor_notification_channel" json:"notification_channels,omitempty"`
}

func (Monitor) TableName() string {
	return "monitors"
}

// NotificationChannel placeholder for cross-module reference
type NotificationChannel struct {
	ID uint64 `gorm:"type:bigint unsigned;primaryKey;autoIncrement" json:"id"`
}

func (NotificationChannel) TableName() string {
	return "notification_channels"
}
