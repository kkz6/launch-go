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
	ID         string                `json:"id"`
	SiteName   string                `json:"site_name"`
	SiteID     string                `json:"site_id"`
	ServerID   string                `json:"server_id"`
	ServerName string                `json:"server_name"`
	Status     string                `json:"status"`
	CreatedAt  *time.Time            `json:"created_at"`
	CommitSha  string                `json:"commit_sha"`
	User       *DashboardUserResponse `json:"user"`
}

// DashboardUserResponse represents a user in dashboard activity
type DashboardUserResponse struct {
	Name string `json:"name"`
}
