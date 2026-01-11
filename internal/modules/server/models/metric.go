package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Metric represents server performance metrics
type Metric struct {
	ID            string    `gorm:"primaryKey;size:26" json:"id"`
	ServerID      string    `gorm:"size:26;not null;index" json:"server_id"`
	CPUUsage      float64   `json:"cpu_usage"`
	MemoryUsage   float64   `json:"memory_usage"`
	DiskUsage     float64   `json:"disk_usage"`
	LoadAverage1  float64   `json:"load_average_1"`
	LoadAverage5  float64   `json:"load_average_5"`
	LoadAverage15 float64   `json:"load_average_15"`
	RecordedAt    time.Time `json:"recorded_at"`
	CreatedAt     time.Time `json:"created_at"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (m *Metric) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = utils.NewULID()
	}

	return nil
}

func (m *Metric) TableName() string {
	return "metrics"
}
