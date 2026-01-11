package dns

import "time"

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
func ToDomainProviderResponse(dp *DomainProvider, domainsCount int) DomainProviderResponse {
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

// CreateDomainRequest represents a request to create a domain
type CreateDomainRequest struct {
	Label    string `json:"label" validate:"required,min=1,max=255"`
	Address  string `json:"address" validate:"required,fqdn"`
	Provider string `json:"provider" validate:"required,ulid"`
}

// DomainResponse represents a domain in API responses
type DomainResponse struct {
	ID               string               `json:"id"`
	Label            string               `json:"label"`
	Address          string               `json:"address"`
	ProviderID       string               `json:"provider_id"`
	DomainProviderID string               `json:"domain_provider_id"`
	Provider         *ProviderSummary     `json:"provider,omitempty"`
	RecordsCount     int                  `json:"records_count"`
	Records          []DnsRecordResponse  `json:"records,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

// ProviderSummary represents a summary of a provider for domain responses
type ProviderSummary struct {
	ID       string `json:"id"`
	Profile  string `json:"profile"`
	Provider string `json:"provider"`
}

// ToDomainResponse converts a Domain to DomainResponse
func ToDomainResponse(d *Domain) DomainResponse {
	resp := DomainResponse{
		ID:               d.ID,
		Label:            d.Label,
		Address:          d.Address,
		ProviderID:       d.ProviderID,
		DomainProviderID: d.DomainProviderID,
		RecordsCount:     len(d.Records),
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}

	if d.Provider != nil {
		resp.Provider = &ProviderSummary{
			ID:       d.Provider.ID,
			Profile:  d.Provider.Profile,
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

// CreateDnsRecordRequest represents a request to create a DNS record
type CreateDnsRecordRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=255"`
	Value    string `json:"value" validate:"required,min=1"`
	Type     string `json:"type" validate:"required,oneof=A AAAA CNAME MX TXT SRV CAA NS"`
	TTL      int    `json:"ttl" validate:"omitempty,min=1"`
	Priority *int   `json:"priority" validate:"omitempty,min=0"`
	Weight   *int   `json:"weight" validate:"omitempty,min=0"`
	Port     *int   `json:"port" validate:"omitempty,min=1,max=65535"`
	Flags    *int   `json:"flags" validate:"omitempty,min=0"`
	Tag      string `json:"tag" validate:"omitempty,max=255"`
	Comment  string `json:"comment" validate:"omitempty,max=1000"`
	Proxied  *bool  `json:"proxied"`
}

// ToModel converts the request to a DnsRecord model
func (r *CreateDnsRecordRequest) ToModel(domainID string) *DnsRecord {
	record := &DnsRecord{
		DomainID: domainID,
		Type:     RecordType(r.Type),
		Name:     r.Name,
		Value:    r.Value,
		TTL:      r.TTL,
		Priority: r.Priority,
		Weight:   r.Weight,
		Port:     r.Port,
		Flags:    r.Flags,
		Proxied:  r.Proxied,
	}

	if r.TTL == 0 {
		record.TTL = 3600
	}

	if r.Tag != "" {
		record.Tag = &r.Tag
	}

	if r.Comment != "" {
		record.Comment = &r.Comment
	}

	return record
}

// UpdateDnsRecordRequest represents a request to update a DNS record
type UpdateDnsRecordRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=255"`
	Value    string `json:"value" validate:"required,min=1"`
	Type     string `json:"type" validate:"required,oneof=A AAAA CNAME MX TXT SRV CAA"`
	TTL      int    `json:"ttl" validate:"omitempty,min=1"`
	Priority *int   `json:"priority" validate:"omitempty,min=0"`
	Weight   *int   `json:"weight" validate:"omitempty,min=0"`
	Port     *int   `json:"port" validate:"omitempty,min=1,max=65535"`
	Flags    *int   `json:"flags" validate:"omitempty,min=0"`
	Tag      string `json:"tag" validate:"omitempty,max=255"`
	Comment  string `json:"comment" validate:"omitempty,max=1000"`
	Proxied  *bool  `json:"proxied"`
}

// ApplyToModel applies the update request to an existing DnsRecord
func (r *UpdateDnsRecordRequest) ApplyToModel(record *DnsRecord) {
	record.Name = r.Name
	record.Value = r.Value
	record.Type = RecordType(r.Type)
	record.Priority = r.Priority
	record.Weight = r.Weight
	record.Port = r.Port
	record.Flags = r.Flags
	record.Proxied = r.Proxied

	if r.TTL > 0 {
		record.TTL = r.TTL
	}

	if r.Tag != "" {
		record.Tag = &r.Tag
	} else {
		record.Tag = nil
	}

	if r.Comment != "" {
		record.Comment = &r.Comment
	} else {
		record.Comment = nil
	}
}

// DnsRecordResponse represents a DNS record in API responses
type DnsRecordResponse struct {
	ID         string  `json:"id"`
	DomainID   string  `json:"domain_id"`
	ProviderID string  `json:"provider_id"`
	Type       string  `json:"type"`
	Name       string  `json:"name"`
	Value      string  `json:"value"`
	TTL        int     `json:"ttl"`
	Priority   *int    `json:"priority,omitempty"`
	Tag        *string `json:"tag,omitempty"`
	Weight     *int    `json:"weight,omitempty"`
	Port       *int    `json:"port,omitempty"`
	Flags      *int    `json:"flags,omitempty"`
	Comment    *string `json:"comment,omitempty"`
	Proxied    *bool   `json:"proxied,omitempty"`
	IsEditable bool    `json:"is_editable"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// ToDnsRecordResponse converts a DnsRecord to DnsRecordResponse
func ToDnsRecordResponse(r *DnsRecord) DnsRecordResponse {
	return DnsRecordResponse{
		ID:         r.ID,
		DomainID:   r.DomainID,
		ProviderID: r.ProviderID,
		Type:       r.Type.String(),
		Name:       r.Name,
		Value:      r.Value,
		TTL:        r.TTL,
		Priority:   r.Priority,
		Tag:        r.Tag,
		Weight:     r.Weight,
		Port:       r.Port,
		Flags:      r.Flags,
		Comment:    r.Comment,
		Proxied:    r.Proxied,
		IsEditable: r.IsEditable(),
		CreatedAt:  r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  r.UpdatedAt.Format(time.RFC3339),
	}
}

// DeleteDomainRequest represents a request to delete a domain
type DeleteDomainRequest struct {
	DeleteFromProvider bool `json:"delete_from_provider"`
}

// SyncDomainsRequest represents a request to sync domains from a provider
type SyncDomainsRequest struct {
	ProviderID string `json:"provider_id" validate:"required,ulid"`
}
