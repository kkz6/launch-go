package providers

import (
	"context"
	"fmt"
	"strings"
)

const (
	digitalOceanAPIBaseURL = "https://api.digitalocean.com/v2"
)

// DigitalOceanProvider implements the Provider interface for DigitalOcean
type DigitalOceanProvider struct {
	*HTTPBaseProvider
}

// NewDigitalOceanProvider creates a new DigitalOceanProvider
func NewDigitalOceanProvider(credentials map[string]string) *DigitalOceanProvider {
	token := ""
	if credentials != nil {
		token = credentials["token"]
	}

	return &DigitalOceanProvider{
		HTTPBaseProvider: NewHTTPBaseProvider(HTTPBaseConfig{
			BaseURL:      digitalOceanAPIBaseURL,
			ProviderName: "DigitalOcean",
			Token:        token,
		}),
	}
}

// Name returns the provider name
func (p *DigitalOceanProvider) Name() string {
	return "DigitalOcean"
}

// SetDomain sets the domain to operate on
func (p *DigitalOceanProvider) SetDomain(domain string) Provider {
	p.HTTPBaseProvider.SetDomain(domain)
	return p
}

// GetDomain returns the current domain
func (p *DigitalOceanProvider) GetDomain() string {
	return p.HTTPBaseProvider.GetDomain()
}

// SetCredentials sets the provider credentials
func (p *DigitalOceanProvider) SetCredentials(credentials map[string]string) Provider {
	p.HTTPBaseProvider.SetCredentials(credentials)
	if token := credentials["token"]; token != "" {
		p.SetToken(token)
	}
	return p
}

// ValidateCredentials validates the DigitalOcean API token
func (p *DigitalOceanProvider) ValidateCredentials(ctx context.Context) error {
	var result map[string]interface{}
	return p.Get(ctx, "/account", &result)
}

// AddDomain adds a new domain to DigitalOcean
func (p *DigitalOceanProvider) AddDomain(ctx context.Context, domainName string) (string, error) {
	p.SetDomain(domainName)

	// Check if domain already exists
	existing, err := p.getExistingDomain(ctx)
	if err == nil && existing != nil {
		// DigitalOcean doesn't provide domain IDs, use domain name
		return domainName, nil
	}

	// Create new domain
	payload := map[string]string{
		"name": domainName,
	}

	var result map[string]interface{}
	if err := p.Post(ctx, "/domains", payload, &result); err != nil {
		return "", err
	}

	return domainName, nil
}

// DeleteDomain deletes a domain from DigitalOcean
func (p *DigitalOceanProvider) DeleteDomain(ctx context.Context, domainName string) error {
	p.SetDomain(domainName)

	// Check if domain exists
	existing, err := p.getExistingDomain(ctx)
	if err != nil {
		return err
	}

	if existing == nil {
		return NewProviderError("DigitalOcean", 404, fmt.Sprintf("domain not found for %s", domainName), nil)
	}

	var result interface{}
	return p.Delete(ctx, fmt.Sprintf("/domains/%s", domainName), &result)
}

// ListDomains lists all domains in DigitalOcean
func (p *DigitalOceanProvider) ListDomains(ctx context.Context) (map[string]string, error) {
	var result doDomainsResponse
	if err := p.GetWithQuery(ctx, "/domains", map[string]string{"per_page": "200"}, &result); err != nil {
		return nil, err
	}

	domains := make(map[string]string)
	for _, d := range result.Domains {
		// DigitalOcean uses domain name as ID
		domains[d.Name] = d.Name
	}

	return domains, nil
}

// GetNameservers returns the nameservers for the current domain
func (p *DigitalOceanProvider) GetNameservers(ctx context.Context) ([]string, error) {
	// DigitalOcean has fixed nameservers
	return []string{
		"ns1.digitalocean.com",
		"ns2.digitalocean.com",
		"ns3.digitalocean.com",
	}, nil
}

// ListRecords lists all DNS records for the current domain
func (p *DigitalOceanProvider) ListRecords(ctx context.Context) ([]ProviderRecord, error) {
	var result doRecordsResponse
	path := fmt.Sprintf("/domains/%s/records", p.GetDomain())
	if err := p.GetWithQuery(ctx, path, map[string]string{"per_page": "200"}, &result); err != nil {
		return nil, err
	}

	var records []ProviderRecord
	for _, r := range result.DomainRecords {
		recordType := strings.ToUpper(r.Type)

		// Skip unsupported record types
		rt, err := ParseRecordType(recordType)
		if err != nil {
			continue
		}

		pr := ProviderRecord{
			ID:    fmt.Sprintf("%d", r.ID),
			Type:  rt,
			Name:  r.Name,
			Value: r.Data,
			TTL:   r.TTL,
		}

		if r.Priority > 0 {
			priority := r.Priority
			pr.Priority = &priority
		}

		if r.Tag != "" {
			pr.Tag = &r.Tag
		}

		if r.Weight > 0 {
			weight := r.Weight
			pr.Weight = &weight
		}

		if r.Port > 0 {
			port := r.Port
			pr.Port = &port
		}

		if r.Flags > 0 {
			flags := r.Flags
			pr.Flags = &flags
		}

		records = append(records, pr)
	}

	return records, nil
}

// AddRecord adds a DNS record to DigitalOcean
func (p *DigitalOceanProvider) AddRecord(ctx context.Context, record *DNSRecord) (string, error) {
	data := BuildRecordData(record)
	data["data"] = p.prepValue(record)

	path := fmt.Sprintf("/domains/%s/records", p.GetDomain())
	var result doRecordResponse
	if err := p.Post(ctx, path, data, &result); err != nil {
		return "", err
	}

	return fmt.Sprintf("%d", result.DomainRecord.ID), nil
}

// UpdateRecord updates a DNS record in DigitalOcean
func (p *DigitalOceanProvider) UpdateRecord(ctx context.Context, record *DNSRecord) error {
	data := BuildRecordData(record)
	data["data"] = p.prepValue(record)

	path := fmt.Sprintf("/domains/%s/records/%s", p.GetDomain(), record.ProviderID)
	var result interface{}
	return p.Put(ctx, path, data, &result)
}

// DeleteRecord deletes a DNS record from DigitalOcean
func (p *DigitalOceanProvider) DeleteRecord(ctx context.Context, record *DNSRecord) error {
	path := fmt.Sprintf("/domains/%s/records/%s", p.GetDomain(), record.ProviderID)
	var result interface{}
	return p.Delete(ctx, path, &result)
}

// getExistingDomain checks if a domain exists
func (p *DigitalOceanProvider) getExistingDomain(ctx context.Context) (map[string]interface{}, error) {
	path := fmt.Sprintf("/domains/%s", p.GetDomain())
	resp, err := p.DoRaw(ctx, "GET", path, nil)
	if err != nil {
		// Check for 404 Not Found
		if providerErr, ok := err.(*ProviderError); ok && providerErr.Code == 404 {
			return nil, nil
		}
		return nil, err
	}

	if !resp.IsSuccess() {
		if resp.StatusCode == 404 {
			return nil, nil
		}
		return nil, NewProviderError("DigitalOcean", resp.StatusCode, resp.String(), nil)
	}

	var result map[string]interface{}
	if err := resp.JSON(&result); err != nil {
		return nil, err
	}

	if domain, ok := result["domain"].(map[string]interface{}); ok {
		return domain, nil
	}

	return nil, nil
}

// prepValue prepares a record value for DigitalOcean
func (p *DigitalOceanProvider) prepValue(record *DNSRecord) string {
	if record.Type == RecordTypeCNAME {
		return WithTrailingDot(record.Value)
	}
	return record.Value
}

// DigitalOcean response types

type doDomainsResponse struct {
	Domains []doDomain `json:"domains"`
}

type doDomain struct {
	Name string `json:"name"`
	TTL  int    `json:"ttl"`
}

type doRecordsResponse struct {
	DomainRecords []doRecord `json:"domain_records"`
}

type doRecordResponse struct {
	DomainRecord doRecord `json:"domain_record"`
}

type doRecord struct {
	ID       int    `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Port     int    `json:"port,omitempty"`
	Flags    int    `json:"flags,omitempty"`
}
