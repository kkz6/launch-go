package dto

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/dns/models"
)

// CreateDomainRequest represents a request to create a domain
type CreateDomainRequest struct {
	Label    string `json:"label" validate:"required,min=1,max=255"`
	Address  string `json:"address" validate:"required,fqdn"`
	Provider string `json:"provider" validate:"required,ulid"`
}

// DeleteDomainRequest represents a request to delete a domain
type DeleteDomainRequest struct {
	DeleteFromProvider bool `json:"delete_from_provider"`
}

// SyncDomainsRequest represents a request to sync domains from a provider
type SyncDomainsRequest struct {
	ProviderID string `json:"provider_id" validate:"required,ulid"`
}

// DomainResponse represents a domain in API responses
type DomainResponse struct {
	ID               string              `json:"id"`
	Label            string              `json:"label"`
	Address          string              `json:"address"`
	ProviderID       string              `json:"provider_id"`
	DomainProviderID string              `json:"domain_provider_id"`
	Provider         *ProviderSummary    `json:"provider,omitempty"`
	RecordsCount     int                 `json:"records_count"`
	Records          []DnsRecordResponse `json:"records,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

// ProviderSummary represents a summary of a provider for domain responses
type ProviderSummary struct {
	ID       string `json:"id"`
	Profile  string `json:"profile"`
	Provider string `json:"provider"`
}

// ToDomainResponse converts a Domain to DomainResponse
func ToDomainResponse(d *models.Domain) DomainResponse {
	var createdAt, updatedAt time.Time
	if d.CreatedAt != nil {
		createdAt = *d.CreatedAt
	}
	if d.UpdatedAt != nil {
		updatedAt = *d.UpdatedAt
	}

	resp := DomainResponse{
		ID:               d.ID,
		Label:            d.Label,
		Address:          d.Address,
		ProviderID:       d.ProviderID,
		DomainProviderID: d.DomainProviderID,
		RecordsCount:     len(d.Records),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}

	if d.Provider != nil {
		profile := ""
		if d.Provider.Profile != nil {
			profile = *d.Provider.Profile
		}
		resp.Provider = &ProviderSummary{
			ID:       d.Provider.ID,
			Profile:  profile,
			Provider: d.Provider.Provider.String(),
		}
	}

	if len(d.Records) > 0 {
		resp.Records = make([]DnsRecordResponse, len(d.Records))
		for i, record := range d.Records {
			resp.Records[i] = ToDnsRecordResponse(&record)
		}
	}

	return resp
}
