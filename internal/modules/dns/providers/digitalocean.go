package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	digitalOceanAPIBaseURL = "https://api.digitalocean.com/v2"
)

// DigitalOceanProvider implements the Provider interface for DigitalOcean
type DigitalOceanProvider struct {
	BaseProvider
	httpClient *http.Client
}

// NewDigitalOceanProvider creates a new DigitalOceanProvider
func NewDigitalOceanProvider(credentials map[string]string) *DigitalOceanProvider {
	p := &DigitalOceanProvider{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	p.SetCredentials(credentials)
	return p
}

// Name returns the provider name
func (p *DigitalOceanProvider) Name() string {
	return "DigitalOcean"
}

// SetDomain sets the domain to operate on
func (p *DigitalOceanProvider) SetDomain(domain string) Provider {
	p.BaseProvider.SetDomain(domain)
	return p
}

// GetDomain returns the current domain
func (p *DigitalOceanProvider) GetDomain() string {
	return p.BaseProvider.GetDomain()
}

// SetCredentials sets the provider credentials
func (p *DigitalOceanProvider) SetCredentials(credentials map[string]string) Provider {
	p.BaseProvider.SetCredentials(credentials)
	return p
}

// ValidateCredentials validates the DigitalOcean API token
func (p *DigitalOceanProvider) ValidateCredentials(ctx context.Context) error {
	req, err := p.newRequest(ctx, http.MethodGet, "/account", nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewProviderError("DigitalOcean", 0, "failed to validate credentials", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.parseErrorResponse(resp)
	}

	return nil
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

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := p.newRequest(ctx, http.MethodPost, "/domains", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", NewProviderError("DigitalOcean", 0, "failed to add domain", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", p.parseErrorResponse(resp)
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

	req, err := p.newRequest(ctx, http.MethodDelete, fmt.Sprintf("/domains/%s", domainName), nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewProviderError("DigitalOcean", 0, "failed to delete domain", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return p.parseErrorResponse(resp)
	}

	return nil
}

// ListDomains lists all domains in DigitalOcean
func (p *DigitalOceanProvider) ListDomains(ctx context.Context) (map[string]string, error) {
	req, err := p.newRequest(ctx, http.MethodGet, "/domains?per_page=200", nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, NewProviderError("DigitalOcean", 0, "failed to list domains", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseErrorResponse(resp)
	}

	var result doDomainsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
	req, err := p.newRequest(ctx, http.MethodGet, fmt.Sprintf("/domains/%s/records?per_page=200", p.domain), nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, NewProviderError("DigitalOcean", 0, "failed to list records", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseErrorResponse(resp)
	}

	var result doRecordsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
func (p *DigitalOceanProvider) AddRecord(ctx context.Context, record *DnsRecord) (string, error) {
	data := map[string]interface{}{
		"type": record.Type.String(),
		"name": record.Name,
		"data": p.prepValue(record),
		"ttl":  record.TTL,
	}

	if record.Priority != nil {
		data["priority"] = *record.Priority
	}

	if record.Weight != nil {
		data["weight"] = *record.Weight
	}

	if record.Port != nil {
		data["port"] = *record.Port
	}

	if record.Flags != nil {
		data["flags"] = *record.Flags
	}

	if record.Tag != nil {
		data["tag"] = *record.Tag
	}

	body, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	req, err := p.newRequest(ctx, http.MethodPost, fmt.Sprintf("/domains/%s/records", p.domain), strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", NewProviderError("DigitalOcean", 0, "failed to add record", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", p.parseErrorResponse(resp)
	}

	var result doRecordResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return fmt.Sprintf("%d", result.DomainRecord.ID), nil
}

// UpdateRecord updates a DNS record in DigitalOcean
func (p *DigitalOceanProvider) UpdateRecord(ctx context.Context, record *DnsRecord) error {
	data := map[string]interface{}{
		"type": record.Type.String(),
		"name": record.Name,
		"data": p.prepValue(record),
		"ttl":  record.TTL,
	}

	if record.Priority != nil {
		data["priority"] = *record.Priority
	}

	if record.Weight != nil {
		data["weight"] = *record.Weight
	}

	if record.Port != nil {
		data["port"] = *record.Port
	}

	if record.Flags != nil {
		data["flags"] = *record.Flags
	}

	if record.Tag != nil {
		data["tag"] = *record.Tag
	}

	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := p.newRequest(ctx, http.MethodPut, fmt.Sprintf("/domains/%s/records/%s", p.domain, record.ProviderID), strings.NewReader(string(body)))
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewProviderError("DigitalOcean", 0, "failed to update record", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.parseErrorResponse(resp)
	}

	return nil
}

// DeleteRecord deletes a DNS record from DigitalOcean
func (p *DigitalOceanProvider) DeleteRecord(ctx context.Context, record *DnsRecord) error {
	req, err := p.newRequest(ctx, http.MethodDelete, fmt.Sprintf("/domains/%s/records/%s", p.domain, record.ProviderID), nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewProviderError("DigitalOcean", 0, "failed to delete record", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return p.parseErrorResponse(resp)
	}

	return nil
}

// getExistingDomain checks if a domain exists
func (p *DigitalOceanProvider) getExistingDomain(ctx context.Context) (map[string]interface{}, error) {
	req, err := p.newRequest(ctx, http.MethodGet, fmt.Sprintf("/domains/%s", p.domain), nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, NewProviderError("DigitalOcean", 0, "failed to get domain", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseErrorResponse(resp)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if domain, ok := result["domain"].(map[string]interface{}); ok {
		return domain, nil
	}

	return nil, nil
}

// prepValue prepares a record value for DigitalOcean
func (p *DigitalOceanProvider) prepValue(record *DnsRecord) string {
	if record.Type == RecordTypeCNAME {
		return WithTrailingDot(record.Value)
	}
	return record.Value
}

// newRequest creates a new HTTP request with DigitalOcean authentication
func (p *DigitalOceanProvider) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := digitalOceanAPIBaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.GetToken())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// parseErrorResponse parses an error response from DigitalOcean
func (p *DigitalOceanProvider) parseErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewProviderError("DigitalOcean", resp.StatusCode, "failed to read error response", err)
	}

	var result doErrorResponse
	if err := json.Unmarshal(body, &result); err == nil && result.Message != "" {
		return NewProviderError("DigitalOcean", resp.StatusCode, result.Message, nil)
	}

	return NewProviderError("DigitalOcean", resp.StatusCode, string(body), nil)
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

type doErrorResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}
