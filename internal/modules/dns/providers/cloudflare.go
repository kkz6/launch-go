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
	cloudflareAPIBaseURL = "https://api.cloudflare.com/client/v4"
)

// CloudflareProvider implements the Provider interface for Cloudflare
type CloudflareProvider struct {
	BaseProvider
	accountID  string
	zoneID     string
	httpClient *http.Client
}

// NewCloudflareProvider creates a new CloudflareProvider
func NewCloudflareProvider(credentials map[string]string, accountID string) *CloudflareProvider {
	p := &CloudflareProvider{
		accountID: accountID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	p.SetCredentials(credentials)
	return p
}

// Name returns the provider name
func (p *CloudflareProvider) Name() string {
	return "Cloudflare"
}

// SetDomain sets the domain to operate on and clears cached zone ID
func (p *CloudflareProvider) SetDomain(domain string) Provider {
	p.BaseProvider.SetDomain(domain)
	p.zoneID = "" // Clear cached zone ID when domain changes
	return p
}

// GetDomain returns the current domain
func (p *CloudflareProvider) GetDomain() string {
	return p.BaseProvider.GetDomain()
}

// SetCredentials sets the provider credentials
func (p *CloudflareProvider) SetCredentials(credentials map[string]string) Provider {
	p.BaseProvider.SetCredentials(credentials)
	return p
}

// ValidateCredentials validates the Cloudflare API token
func (p *CloudflareProvider) ValidateCredentials(ctx context.Context) error {
	req, err := p.newRequest(ctx, http.MethodGet, "/user/tokens/verify", nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewProviderError("Cloudflare", 0, "failed to validate credentials", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.parseErrorResponse(resp)
	}

	return nil
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

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := p.newRequest(ctx, http.MethodPost, "/zones", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", NewProviderError("Cloudflare", 0, "failed to add domain", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return "", NewProviderError("Cloudflare", 403, "API key does not have permission to create a zone. Please ensure the API key has the 'Zone:Zone:Edit' permission.", nil)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", p.parseErrorResponse(resp)
	}

	var result cloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
	req, err := p.newRequest(ctx, http.MethodDelete, fmt.Sprintf("/zones/%s", zoneID), nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewProviderError("Cloudflare", 0, "failed to delete domain", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.parseErrorResponse(resp)
	}

	return nil
}

// ListDomains lists all domains (zones) in Cloudflare
func (p *CloudflareProvider) ListDomains(ctx context.Context) (map[string]string, error) {
	req, err := p.newRequest(ctx, http.MethodGet, "/zones?per_page=200", nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, NewProviderError("Cloudflare", 0, "failed to list domains", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseErrorResponse(resp)
	}

	var result cloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
		return nil, NewProviderError("Cloudflare", 404, fmt.Sprintf("zone not found for %s", p.domain), nil)
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

	req, err := p.newRequest(ctx, http.MethodGet, fmt.Sprintf("/zones/%s/dns_records?per_page=1000", zoneID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, NewProviderError("Cloudflare", 0, "failed to list records", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseErrorResponse(resp)
	}

	var result cloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
				p := int(priority)
				pr.Priority = &p
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
				Name:    p.domain,
				Value:   ns,
				TTL:     86400,
				Proxied: boolPtr(false),
			})
		}
	}

	return records, nil
}

// AddRecord adds a DNS record to Cloudflare
func (p *CloudflareProvider) AddRecord(ctx context.Context, record *DnsRecord) (string, error) {
	zoneID, err := p.getZoneID(ctx)
	if err != nil {
		return "", err
	}

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

	body, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	req, err := p.newRequest(ctx, http.MethodPost, fmt.Sprintf("/zones/%s/dns_records", zoneID), strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", NewProviderError("Cloudflare", 0, "failed to add record", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", p.parseErrorResponse(resp)
	}

	var result cloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
func (p *CloudflareProvider) UpdateRecord(ctx context.Context, record *DnsRecord) error {
	zoneID, err := p.getZoneID(ctx)
	if err != nil {
		return err
	}

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

	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := p.newRequest(ctx, http.MethodPut, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, record.ProviderID), strings.NewReader(string(body)))
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewProviderError("Cloudflare", 0, "failed to update record", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.parseErrorResponse(resp)
	}

	return nil
}

// DeleteRecord deletes a DNS record from Cloudflare
func (p *CloudflareProvider) DeleteRecord(ctx context.Context, record *DnsRecord) error {
	zoneID, err := p.getZoneID(ctx)
	if err != nil {
		return err
	}

	req, err := p.newRequest(ctx, http.MethodDelete, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, record.ProviderID), nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewProviderError("Cloudflare", 0, "failed to delete record", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.parseErrorResponse(resp)
	}

	return nil
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
		return "", NewProviderError("Cloudflare", 404, fmt.Sprintf("zone not found for %s", p.domain), nil)
	}

	p.zoneID = zone["id"].(string)
	return p.zoneID, nil
}

// getZoneByDomain finds a zone by domain name
func (p *CloudflareProvider) getZoneByDomain(ctx context.Context) (map[string]interface{}, error) {
	req, err := p.newRequest(ctx, http.MethodGet, fmt.Sprintf("/zones?name=%s", p.domain), nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, NewProviderError("Cloudflare", 0, "failed to get zone", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseErrorResponse(resp)
	}

	var result cloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if zones, ok := result.Result.([]interface{}); ok && len(zones) > 0 {
		return zones[0].(map[string]interface{}), nil
	}

	return nil, nil
}

// newRequest creates a new HTTP request with Cloudflare authentication
func (p *CloudflareProvider) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := cloudflareAPIBaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.GetToken())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// parseErrorResponse parses an error response from Cloudflare
func (p *CloudflareProvider) parseErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var result cloudflareResponse
	if err := json.Unmarshal(body, &result); err == nil && len(result.Errors) > 0 {
		return NewProviderError("Cloudflare", resp.StatusCode, result.Errors[0].Message, nil)
	}

	return NewProviderError("Cloudflare", resp.StatusCode, string(body), nil)
}

// cloudflareResponse represents a response from the Cloudflare API
type cloudflareResponse struct {
	Success bool                     `json:"success"`
	Result  interface{}              `json:"result"`
	Errors  []cloudflareError        `json:"errors"`
}

// cloudflareError represents an error from the Cloudflare API
type cloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Helper function to create a pointer to a bool
func boolPtr(b bool) *bool {
	return &b
}
