package dto

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/dns/models"
)

// CreateDomainProviderRequest represents a request to create a DNS provider
type CreateDomainProviderRequest struct {
	Profile   string `json:"profile" validate:"required,min=1,max=255"`
	Provider  string `json:"provider" validate:"required,oneof=cloudflare digitalocean"`
	Token     string `json:"token" validate:"required,min=1"`
	AccountID string `json:"account_id" validate:"required_if=Provider cloudflare"`
}

// UpdateDomainProviderRequest represents a request to update a DNS provider
type UpdateDomainProviderRequest struct {
	Profile string `json:"profile" validate:"required,min=1,max=255"`
}

// DomainProviderResponse represents a DNS provider in API responses
type DomainProviderResponse struct {
	ID               string     `json:"id"`
	Profile          string     `json:"profile"`
	Provider         string     `json:"provider"`
	ProviderLabel    string     `json:"provider_label"`
	Connected        bool       `json:"connected"`
	SyncStatus       string     `json:"sync_status,omitempty"`
	LastSyncedAt     *time.Time `json:"last_synced_at,omitempty"`
	SyncErrorMessage *string    `json:"sync_error_message,omitempty"`
	DomainsCount     int        `json:"domains_count"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// ToDomainProviderResponse converts a DomainProvider to DomainProviderResponse
func ToDomainProviderResponse(dp *models.DomainProvider, domainsCount int) DomainProviderResponse {
	return DomainProviderResponse{
		ID:               dp.ID,
		Profile:          dp.Profile,
		Provider:         dp.Provider.String(),
		ProviderLabel:    dp.ProviderLabel(),
		Connected:        dp.Connected,
		SyncStatus:       dp.SyncStatus.String(),
		LastSyncedAt:     dp.LastSyncedAt,
		SyncErrorMessage: dp.SyncErrorMessage,
		DomainsCount:     domainsCount,
		CreatedAt:        dp.CreatedAt,
		UpdatedAt:        dp.UpdatedAt,
	}
}
