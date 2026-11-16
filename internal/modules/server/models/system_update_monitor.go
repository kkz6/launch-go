package models

// SystemUpdateMonitor tracks observed state and notifications for a managed server.
type SystemUpdateMonitor struct {
	ServerID  string `gorm:"primaryKey"`
	LastRunID string
	RebootID  string
	StateHash string
}

func (SystemUpdateMonitor) TableName() string {
	return "server_system_update_monitors"
}
