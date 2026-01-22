package dto

import (
	"time"
)

// DashboardResponse represents the dashboard API response
type DashboardResponse struct {
	Servers        []*DashboardServerResponse   `json:"servers"`
	RecentActivity []*DashboardActivityResponse `json:"recent_activity"`
}

// DashboardServerResponse represents a server in the dashboard
type DashboardServerResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"` // "connected" or "disconnected"
	Provider   string `json:"provider"`
	SitesCount int64  `json:"sites_count"`
}

// DashboardActivityResponse represents a recent deployment in the dashboard
type DashboardActivityResponse struct {
	ID            string                 `json:"id"`
	SiteName      string                 `json:"site_name"`
	SiteID        string                 `json:"site_id"`
	ServerID      string                 `json:"server_id"`
	ServerName    string                 `json:"server_name"`
	Status        string                 `json:"status"`
	CreatedAt     *time.Time             `json:"created_at"`
	CommitSha     string                 `json:"commit_sha"`
	CommitMessage string                 `json:"commit_message"`
	User          *DashboardUserResponse `json:"user"`
}

// DashboardUserResponse represents a user in dashboard activity
type DashboardUserResponse struct {
	Name string `json:"name"`
}

// OnboardingStatusResponse represents the onboarding status API response
type OnboardingStatusResponse struct {
	Onboarded              bool `json:"onboarded"`
	HasServerProvider      bool `json:"has_server_provider"`
	HasSourceControl       bool `json:"has_source_control"`
	HasDomainProvider      bool `json:"has_domain_provider"`
	HasStorageProvider     bool `json:"has_storage_provider"`
	HasNotificationChannel bool `json:"has_notification_channel"`
}
