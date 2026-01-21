package providers

import (
	"context"
	"fmt"
	"strings"
)

const (
	cloudflareAPIBaseURL = "https://api.cloudflare.com/client/v4"
)

// CloudflareProvider implements the Provider interface for Cloudflare
type CloudflareProvider struct {
	*HTTPBaseProvider
	accountID string
	zoneID    string
}

// NewCloudflareProvider creates a new CloudflareProvider
func NewCloudflareProvider(credentials map[string]string, accountID string) *CloudflareProvider {
	token := ""
	if credentials != nil {
		token = credentials["token"]
	}

	return &CloudflareProvider{
		HTTPBaseProvider: NewHTTPBaseProvider(HTTPBaseConfig{
			BaseURL:      cloudflareAPIBaseURL,
			ProviderName: "Cloudflare",
			Token:        token,
		}),
		accountID: accountID,
	}
}

// Name returns the provider name
func (p *CloudflareProvider) Name() string {
	return "Cloudflare"
}

// SetDomain sets the domain to operate on and clears cached zone ID
func (p *CloudflareProvider) SetDomain(domain string) Provider {
	p.HTTPBaseProvider.SetDomain(domain)
	p.zoneID = "" // Clear cached zone ID when domain changes
	return p
}

// GetDomain returns the current domain
func (p *CloudflareProvider) GetDomain() string {
	return p.HTTPBaseProvider.GetDomain()
}

// SetCredentials sets the provider credentials
func (p *CloudflareProvider) SetCredentials(credentials map[string]string) Provider {
	p.HTTPBaseProvider.SetCredentials(credentials)
	if token := credentials["token"]; token != "" {
		p.SetToken(token)
	}
	return p
}

// ValidateCredentials validates the Cloudflare API token
func (p *CloudflareProvider) ValidateCredentials(ctx context.Context) error {
	var result cloudflareResponse
	return p.Get(ctx, "/user/tokens/verify", &result)
}

// AddDomain adds a new domain (zone) to Cloudflare
func (p *CloudflareProvider) AddDomain(ctx context.Context, domainName string) (string, error) {
	p.SetDomain(domainName)

	// Check if zone already exists
	zone, err := p.getZoneByDomain(ctx)
	if err == nil && zone != nil {
		return zone["id"].(string), nil
	}

	// Create new zone
	payload := map[string]interface{}{
		"name": domainName,
		"account": map[string]string{
			"id": p.accountID,
		},
	}

	var result cloudflareResponse
	if err := p.Post(ctx, "/zones", payload, &result); err != nil {
		// Check for forbidden error
		if providerErr, ok := err.(*ProviderError); ok && providerErr.Code == 403 {
			return "", NewProviderError("Cloudflare", 403, "API key does not have permission to create a zone. Please ensure the API key has the 'Zone:Zone:Edit' permission.", nil)
		}
		return "", err
	}

	if resultMap, ok := result.Result.(map[string]interface{}); ok {
		if id, ok := resultMap["id"].(string); ok {
			return id, nil
		}
	}

	return "", NewProviderError("Cloudflare", 0, "unexpected response format", nil)
}

// DeleteDomain deletes a domain (zone) from Cloudflare
func (p *CloudflareProvider) DeleteDomain(ctx context.Context, domainName string) error {
	p.SetDomain(domainName)

	zone, err := p.getZoneByDomain(ctx)
	if err != nil {
		return err
	}

	if zone == nil {
		return NewProviderError("Cloudflare", 404, fmt.Sprintf("zone not found for %s", domainName), nil)
	}

	zoneID := zone["id"].(string)
	var result interface{}
	return p.Delete(ctx, fmt.Sprintf("/zones/%s", zoneID), &result)
}

// ListDomains lists all domains (zones) in Cloudflare
func (p *CloudflareProvider) ListDomains(ctx context.Context) (map[string]string, error) {
	var result cloudflareResponse
	if err := p.GetWithQuery(ctx, "/zones", map[string]string{"per_page": "200"}, &result); err != nil {
		return nil, err
	}

	domains := make(map[string]string)
	if zones, ok := result.Result.([]interface{}); ok {
		for _, z := range zones {
			if zone, ok := z.(map[string]interface{}); ok {
				id := zone["id"].(string)
				name := zone["name"].(string)
				domains[id] = name
			}
		}
	}

	return domains, nil
}

// GetNameservers returns the nameservers for the current domain
func (p *CloudflareProvider) GetNameservers(ctx context.Context) ([]string, error) {
	zone, err := p.getZoneByDomain(ctx)
	if err != nil {
		return nil, err
	}

	if zone == nil {
		return nil, NewProviderError("Cloudflare", 404, fmt.Sprintf("zone not found for %s", p.GetDomain()), nil)
	}

	if ns, ok := zone["name_servers"].([]interface{}); ok {
		nameservers := make([]string, len(ns))
		for i, n := range ns {
			nameservers[i] = n.(string)
		}
		return nameservers, nil
	}

	return nil, NewProviderError("Cloudflare", 0, "nameservers not found in zone response", nil)
}

// ListRecords lists all DNS records for the current domain
func (p *CloudflareProvider) ListRecords(ctx context.Context) ([]ProviderRecord, error) {
	zoneID, err := p.getZoneID(ctx)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/zones/%s/dns_records", zoneID)
	var result cloudflareResponse
	if err := p.GetWithQuery(ctx, path, map[string]string{"per_page": "1000"}, &result); err != nil {
		return nil, err
	}

	var records []ProviderRecord
	if results, ok := result.Result.([]interface{}); ok {
		for _, r := range results {
			record := r.(map[string]interface{})
			recordType := strings.ToUpper(record["type"].(string))

			// Skip unsupported record types
			rt, err := ParseRecordType(recordType)
			if err != nil {
				continue
			}

			pr := ProviderRecord{
				ID:    record["id"].(string),
				Type:  rt,
				Name:  record["name"].(string),
				Value: record["content"].(string),
				TTL:   int(record["ttl"].(float64)),
			}

			if priority, ok := record["priority"].(float64); ok {
				pri := int(priority)
				pr.Priority = &pri
			}

			if comment, ok := record["comment"].(string); ok && comment != "" {
				pr.Comment = &comment
			}

			if proxied, ok := record["proxied"].(bool); ok {
				pr.Proxied = &proxied
			}

			records = append(records, pr)
		}
	}

	// Add NS records from zone nameservers
	nameservers, err := p.GetNameservers(ctx)
	if err == nil {
		for i, ns := range nameservers {
			records = append(records, ProviderRecord{
				ID:      fmt.Sprintf("ns-%d", i),
				Type:    RecordTypeNS,
				Name:    p.GetDomain(),
				Value:   ns,
				TTL:     86400,
				Proxied: BoolPtr(false),
			})
		}
	}

	return records, nil
}

// AddRecord adds a DNS record to Cloudflare
func (p *CloudflareProvider) AddRecord(ctx context.Context, record *DNSRecord) (string, error) {
	zoneID, err := p.getZoneID(ctx)
	if err != nil {
		return "", err
	}

	data := p.buildRecordData(record)
	path := fmt.Sprintf("/zones/%s/dns_records", zoneID)

	var result cloudflareResponse
	if err := p.Post(ctx, path, data, &result); err != nil {
		return "", err
	}

	if resultMap, ok := result.Result.(map[string]interface{}); ok {
		if id, ok := resultMap["id"].(string); ok {
			return id, nil
		}
	}

	return "", nil
}

// UpdateRecord updates a DNS record in Cloudflare
func (p *CloudflareProvider) UpdateRecord(ctx context.Context, record *DNSRecord) error {
	zoneID, err := p.getZoneID(ctx)
	if err != nil {
		return err
	}

	data := p.buildRecordData(record)
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, record.ProviderID)

	var result interface{}
	return p.Put(ctx, path, data, &result)
}

// DeleteRecord deletes a DNS record from Cloudflare
func (p *CloudflareProvider) DeleteRecord(ctx context.Context, record *DNSRecord) error {
	zoneID, err := p.getZoneID(ctx)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, record.ProviderID)
	var result interface{}
	return p.Delete(ctx, path, &result)
}

// getZoneID returns the zone ID for the current domain
func (p *CloudflareProvider) getZoneID(ctx context.Context) (string, error) {
	if p.zoneID != "" {
		return p.zoneID, nil
	}

	zone, err := p.getZoneByDomain(ctx)
	if err != nil {
		return "", err
	}

	if zone == nil {
		return "", NewProviderError("Cloudflare", 404, fmt.Sprintf("zone not found for %s", p.GetDomain()), nil)
	}

	p.zoneID = zone["id"].(string)
	return p.zoneID, nil
}

// getZoneByDomain finds a zone by domain name
func (p *CloudflareProvider) getZoneByDomain(ctx context.Context) (map[string]interface{}, error) {
	var result cloudflareResponse
	if err := p.GetWithQuery(ctx, "/zones", map[string]string{"name": p.GetDomain()}, &result); err != nil {
		return nil, err
	}

	if zones, ok := result.Result.([]interface{}); ok && len(zones) > 0 {
		return zones[0].(map[string]interface{}), nil
	}

	return nil, nil
}

// buildRecordData builds the request data for Cloudflare DNS records
func (p *CloudflareProvider) buildRecordData(record *DNSRecord) map[string]interface{} {
	data := map[string]interface{}{
		"type":    record.Type.String(),
		"name":    record.Name,
		"content": record.Value,
	}

	// Handle TTL for proxied records
	if record.Type.SupportsProxy() && record.Proxied != nil && *record.Proxied {
		data["ttl"] = 1 // Auto TTL for proxied records
	} else {
		if record.TTL > 0 {
			data["ttl"] = record.TTL
		} else {
			data["ttl"] = 3600
		}
	}

	// Add proxied flag for supported record types
	if record.Type.SupportsProxy() && record.Proxied != nil {
		data["proxied"] = *record.Proxied
	}

	if record.Priority != nil {
		data["priority"] = *record.Priority
	}

	if record.Comment != nil && *record.Comment != "" {
		data["comment"] = *record.Comment
	}

	return data
}

// cloudflareResponse represents a response from the Cloudflare API
type cloudflareResponse struct {
	Success bool              `json:"success"`
	Result  interface{}       `json:"result"`
	Errors  []cloudflareError `json:"errors"`
}

// cloudflareError represents an error from the Cloudflare API
type cloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
