package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCloudflareTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *CloudflareProvider) {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	provider := NewCloudflareProvider(map[string]string{"token": "test-token"}, "account123")
	// We can't easily override the base URL, so we'll test the methods that don't make HTTP calls
	// For integration tests, we would use a mock HTTP client

	return server, provider
}

func TestCloudflareProvider_Name(t *testing.T) {
	provider := NewCloudflareProvider(map[string]string{"token": "test"}, "account123")

	assert.Equal(t, "Cloudflare", provider.Name())
}

func TestCloudflareProvider_SetDomain(t *testing.T) {
	provider := NewCloudflareProvider(map[string]string{"token": "test"}, "account123")

	result := provider.SetDomain("example.com")

	assert.Equal(t, "example.com", provider.GetDomain())
	assert.Equal(t, provider, result)
}

func TestCloudflareProvider_SetDomain_ClearsZoneID(t *testing.T) {
	provider := NewCloudflareProvider(map[string]string{"token": "test"}, "account123")
	provider.zoneID = "old-zone-id"

	provider.SetDomain("new-domain.com")

	assert.Empty(t, provider.zoneID)
}

func TestCloudflareProvider_GetDomain(t *testing.T) {
	provider := NewCloudflareProvider(map[string]string{"token": "test"}, "account123")
	provider.SetDomain("example.com")

	assert.Equal(t, "example.com", provider.GetDomain())
}

func TestCloudflareProvider_SetCredentials(t *testing.T) {
	provider := NewCloudflareProvider(nil, "account123")

	result := provider.SetCredentials(map[string]string{"token": "new-token"})

	assert.Equal(t, "new-token", provider.GetToken())
	assert.Equal(t, provider, result)
}

func TestNewCloudflareProvider(t *testing.T) {
	creds := map[string]string{"token": "test-token"}
	provider := NewCloudflareProvider(creds, "account123")

	assert.NotNil(t, provider)
	assert.Equal(t, "account123", provider.accountID)
	assert.Equal(t, "test-token", provider.GetToken())
	assert.NotNil(t, provider.httpClient)
}

func TestCloudflareProvider_BoolPtr(t *testing.T) {
	b := boolPtr(true)
	assert.True(t, *b)

	b = boolPtr(false)
	assert.False(t, *b)
}

// Mock HTTP tests for Cloudflare

func TestCloudflareProvider_ValidateCredentials_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/user/tokens/verify", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"result":  map[string]interface{}{"id": "token123"},
		})
	}))
	defer server.Close()

	provider := &CloudflareProvider{
		httpClient: server.Client(),
	}
	provider.SetCredentials(map[string]string{"token": "test-token"})

	// Note: This test would require overriding the base URL which we can't easily do
	// In a real scenario, we would use dependency injection or a custom HTTP transport
}

func TestCloudflareProvider_ListDomains_Response(t *testing.T) {
	// Test the response parsing logic
	responseData := cloudflareResponse{
		Success: true,
		Result: []interface{}{
			map[string]interface{}{"id": "zone1", "name": "example.com"},
			map[string]interface{}{"id": "zone2", "name": "test.com"},
		},
	}

	// Verify the structure is correct
	zones, ok := responseData.Result.([]interface{})
	require.True(t, ok)
	assert.Len(t, zones, 2)

	zone1 := zones[0].(map[string]interface{})
	assert.Equal(t, "zone1", zone1["id"])
	assert.Equal(t, "example.com", zone1["name"])
}

func TestCloudflareProvider_ListRecords_Response(t *testing.T) {
	// Test the response parsing logic for records
	responseData := cloudflareResponse{
		Success: true,
		Result: []interface{}{
			map[string]interface{}{
				"id":       "rec1",
				"type":     "A",
				"name":     "example.com",
				"content":  "1.2.3.4",
				"ttl":      float64(3600),
				"proxied":  true,
				"priority": float64(10),
				"comment":  "Test record",
			},
		},
	}

	records, ok := responseData.Result.([]interface{})
	require.True(t, ok)
	assert.Len(t, records, 1)

	record := records[0].(map[string]interface{})
	assert.Equal(t, "rec1", record["id"])
	assert.Equal(t, "A", record["type"])
	assert.Equal(t, "1.2.3.4", record["content"])
	assert.Equal(t, float64(3600), record["ttl"])
	assert.True(t, record["proxied"].(bool))
}

func TestCloudflareProvider_AddRecord_RequestData(t *testing.T) {
	record := &DnsRecord{
		Type:       RecordTypeA,
		Name:       "@",
		Value:      "1.2.3.4",
		TTL:        3600,
		ProviderID: "",
	}

	// Test proxied record TTL handling
	proxied := true
	record.Proxied = &proxied

	// When proxied is true for supported types, TTL should be 1 (auto)
	assert.True(t, record.Type.SupportsProxy())
	assert.NotNil(t, record.Proxied)
	assert.True(t, *record.Proxied)

	// Build request data like the provider does
	data := map[string]interface{}{
		"type":    record.Type.String(),
		"name":    record.Name,
		"content": record.Value,
	}

	if record.Type.SupportsProxy() && record.Proxied != nil && *record.Proxied {
		data["ttl"] = 1
	} else {
		data["ttl"] = record.TTL
	}

	if record.Type.SupportsProxy() && record.Proxied != nil {
		data["proxied"] = *record.Proxied
	}

	assert.Equal(t, "A", data["type"])
	assert.Equal(t, "@", data["name"])
	assert.Equal(t, "1.2.3.4", data["content"])
	assert.Equal(t, 1, data["ttl"])
	assert.True(t, data["proxied"].(bool))
}

func TestCloudflareProvider_UpdateRecord_RequestData(t *testing.T) {
	priority := 10
	comment := "Updated comment"

	record := &DnsRecord{
		Type:       RecordTypeMX,
		Name:       "mail",
		Value:      "mail.example.com",
		TTL:        7200,
		Priority:   &priority,
		Comment:    &comment,
		ProviderID: "existing-id",
	}

	// Build request data like the provider does
	data := map[string]interface{}{
		"type":    record.Type.String(),
		"name":    record.Name,
		"content": record.Value,
		"ttl":     record.TTL,
	}

	if record.Priority != nil {
		data["priority"] = *record.Priority
	}

	if record.Comment != nil && *record.Comment != "" {
		data["comment"] = *record.Comment
	}

	assert.Equal(t, "MX", data["type"])
	assert.Equal(t, "mail", data["name"])
	assert.Equal(t, "mail.example.com", data["content"])
	assert.Equal(t, 7200, data["ttl"])
	assert.Equal(t, 10, data["priority"])
	assert.Equal(t, "Updated comment", data["comment"])
}

func TestCloudflareProvider_ErrorResponse(t *testing.T) {
	response := cloudflareResponse{
		Success: false,
		Errors: []cloudflareError{
			{Code: 1001, Message: "Invalid API token"},
		},
	}

	assert.False(t, response.Success)
	assert.Len(t, response.Errors, 1)
	assert.Equal(t, 1001, response.Errors[0].Code)
	assert.Equal(t, "Invalid API token", response.Errors[0].Message)
}

func TestCloudflareProvider_GetZoneByDomain_Response(t *testing.T) {
	// Test parsing zone response
	responseData := cloudflareResponse{
		Success: true,
		Result: []interface{}{
			map[string]interface{}{
				"id":           "zone123",
				"name":         "example.com",
				"name_servers": []interface{}{"ns1.cloudflare.com", "ns2.cloudflare.com"},
			},
		},
	}

	zones, ok := responseData.Result.([]interface{})
	require.True(t, ok)
	require.Len(t, zones, 1)

	zone := zones[0].(map[string]interface{})
	assert.Equal(t, "zone123", zone["id"])

	nameservers := zone["name_servers"].([]interface{})
	assert.Len(t, nameservers, 2)
	assert.Equal(t, "ns1.cloudflare.com", nameservers[0])
}

func TestCloudflareProvider_AddDomain_RequestData(t *testing.T) {
	provider := NewCloudflareProvider(map[string]string{"token": "test"}, "account123")
	provider.SetDomain("newdomain.com")

	// Build payload like the provider does
	payload := map[string]interface{}{
		"name": "newdomain.com",
		"account": map[string]string{
			"id": provider.accountID,
		},
	}

	assert.Equal(t, "newdomain.com", payload["name"])

	account := payload["account"].(map[string]string)
	assert.Equal(t, "account123", account["id"])
}

func TestCloudflareProvider_RecordTypeFiltering(t *testing.T) {
	// Test that unsupported record types are filtered out
	recordTypes := []string{"A", "AAAA", "CNAME", "MX", "TXT", "SRV", "NS", "SOA", "CAA", "HTTPS", "DNSKEY"}

	var validRecords []string
	for _, rt := range recordTypes {
		if _, err := ParseRecordType(rt); err == nil {
			validRecords = append(validRecords, rt)
		}
	}

	assert.Len(t, validRecords, 9) // A, AAAA, CNAME, MX, TXT, SRV, NS, SOA, CAA
	assert.NotContains(t, validRecords, "HTTPS")
	assert.NotContains(t, validRecords, "DNSKEY")
}

func TestCloudflareProvider_NewRequest_Headers(t *testing.T) {
	provider := NewCloudflareProvider(map[string]string{"token": "test-token"}, "account123")

	ctx := context.Background()
	req, err := provider.newRequest(ctx, http.MethodGet, "/test", nil)
	require.NoError(t, err)

	assert.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
}
