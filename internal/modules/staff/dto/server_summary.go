package dto

import (
	"time"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
)

// ServerSummary is the explicit allow-list view of a server returned by the
// back-office /admin/servers endpoint. The raw Server model already tags every
// secret field (private key, host key, provider data, launch token, passwords)
// as json:"-", but the staff endpoint is cross-tenant, so this DTO is a
// defense-in-depth allow-list: only fields enumerated here ever serialize.
// A future sensitive field added to Server cannot leak through this endpoint
// unless it is deliberately added below.
type ServerSummary struct {
	ID              string     `json:"id"`
	TeamID          string     `json:"team_id"`
	UserID          string     `json:"user_id"`
	Name            string     `json:"name"`
	Description     *string    `json:"description,omitempty"`
	Provider        string     `json:"provider"`
	Type            *string    `json:"type,omitempty"`
	Status          string     `json:"status"`
	PublicIPv4      *string    `json:"public_ipv4,omitempty"`
	Connected       bool       `json:"connected"`
	OperatingSystem *string    `json:"operating_system,omitempty"`
	CPUCores        *int       `json:"cpu_cores,omitempty"`
	MemoryInMB      *int       `json:"memory_in_mb,omitempty"`
	StorageInGB     *int       `json:"storage_in_gb,omitempty"`
	ProvisionedAt   *time.Time `json:"provisioned_at,omitempty"`
	ArchivedAt      *time.Time `json:"archived_at,omitempty"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}

// NewServerSummary maps a Server model to its safe summary view.
func NewServerSummary(s servermodels.Server) ServerSummary {
	return ServerSummary{
		ID:              s.ID,
		TeamID:          s.TeamID,
		UserID:          s.UserID,
		Name:            s.Name,
		Description:     s.Description,
		Provider:        s.Provider.String(),
		Type:            s.Type,
		Status:          s.Status.String(),
		PublicIPv4:      s.PublicIPv4,
		Connected:       s.Connected,
		OperatingSystem: s.OperatingSystem,
		CPUCores:        s.CPUCores,
		MemoryInMB:      s.MemoryInMB,
		StorageInGB:     s.StorageInGB,
		ProvisionedAt:   s.ProvisionedAt,
		ArchivedAt:      s.ArchivedAt,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
}

// NewServerSummaries maps a slice of Server models to summary views.
func NewServerSummaries(servers []servermodels.Server) []ServerSummary {
	summaries := make([]ServerSummary, 0, len(servers))
	for _, s := range servers {
		summaries = append(summaries, NewServerSummary(s))
	}

	return summaries
}
