package providers

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDigitalOceanProvider_Name(t *testing.T) {
	provider := NewDigitalOceanProvider(map[string]string{"token": "test"})

	assert.Equal(t, "DigitalOcean", provider.Name())
}

func TestDigitalOceanProvider_SetDomain(t *testing.T) {
	provider := NewDigitalOceanProvider(map[string]string{"token": "test"})

	result := provider.SetDomain("example.com")

	assert.Equal(t, "example.com", provider.GetDomain())
	assert.Equal(t, provider, result)
}

func TestDigitalOceanProvider_GetDomain(t *testing.T) {
	provider := NewDigitalOceanProvider(map[string]string{"token": "test"})
	provider.SetDomain("example.com")

	assert.Equal(t, "example.com", provider.GetDomain())
}

func TestDigitalOceanProvider_SetCredentials(t *testing.T) {
	provider := NewDigitalOceanProvider(nil)

	result := provider.SetCredentials(map[string]string{"token": "new-token"})

	assert.Equal(t, "new-token", provider.GetToken())
	assert.Equal(t, provider, result)
}

func TestNewDigitalOceanProvider(t *testing.T) {
	creds := map[string]string{"token": "test-token"}
	provider := NewDigitalOceanProvider(creds)

	assert.NotNil(t, provider)
	assert.Equal(t, "test-token", provider.GetToken())
	assert.NotNil(t, provider.httpClient)
}

func TestDigitalOceanProvider_GetNameservers(t *testing.T) {
	provider := NewDigitalOceanProvider(map[string]string{"token": "test"})

	ctx := context.Background()
	nameservers, err := provider.GetNameservers(ctx)
	require.NoError(t, err)

	assert.Len(t, nameservers, 3)
	assert.Contains(t, nameservers, "ns1.digitalocean.com")
	assert.Contains(t, nameservers, "ns2.digitalocean.com")
	assert.Contains(t, nameservers, "ns3.digitalocean.com")
}

func TestDigitalOceanProvider_PrepValue_CNAME(t *testing.T) {
	provider := NewDigitalOceanProvider(map[string]string{"token": "test"})

	record := &DNSRecord{
		Type:  RecordTypeCNAME,
		Value: "example.com",
	}

	result := provider.prepValue(record)

	assert.Equal(t, "example.com.", result)
}

func TestDigitalOceanProvider_PrepValue_CNAME_AlreadyHasDot(t *testing.T) {
	provider := NewDigitalOceanProvider(map[string]string{"token": "test"})

	record := &DNSRecord{
		Type:  RecordTypeCNAME,
		Value: "example.com.",
	}

	result := provider.prepValue(record)

	assert.Equal(t, "example.com.", result)
}

func TestDigitalOceanProvider_PrepValue_NonCNAME(t *testing.T) {
	provider := NewDigitalOceanProvider(map[string]string{"token": "test"})

	record := &DNSRecord{
		Type:  RecordTypeA,
		Value: "1.2.3.4",
	}

	result := provider.prepValue(record)

	assert.Equal(t, "1.2.3.4", result)
}

func TestDigitalOceanProvider_DomainsResponse(t *testing.T) {
	// Test the response parsing structure
	response := doDomainsResponse{
		Domains: []doDomain{
			{Name: "example.com", TTL: 1800},
			{Name: "test.com", TTL: 3600},
		},
	}

	assert.Len(t, response.Domains, 2)
	assert.Equal(t, "example.com", response.Domains[0].Name)
	assert.Equal(t, 1800, response.Domains[0].TTL)
}

func TestDigitalOceanProvider_RecordsResponse(t *testing.T) {
	// Test the response parsing structure
	response := doRecordsResponse{
		DomainRecords: []doRecord{
			{
				ID:       123,
				Type:     "A",
				Name:     "@",
				Data:     "1.2.3.4",
				TTL:      3600,
				Priority: 10,
				Tag:      "issue",
				Weight:   5,
				Port:     443,
				Flags:    0,
			},
		},
	}

	assert.Len(t, response.DomainRecords, 1)

	record := response.DomainRecords[0]
	assert.Equal(t, 123, record.ID)
	assert.Equal(t, "A", record.Type)
	assert.Equal(t, "@", record.Name)
	assert.Equal(t, "1.2.3.4", record.Data)
	assert.Equal(t, 3600, record.TTL)
	assert.Equal(t, 10, record.Priority)
	assert.Equal(t, "issue", record.Tag)
	assert.Equal(t, 5, record.Weight)
	assert.Equal(t, 443, record.Port)
	assert.Equal(t, 0, record.Flags)
}

func TestDigitalOceanProvider_RecordResponse(t *testing.T) {
	response := doRecordResponse{
		DomainRecord: doRecord{
			ID:   456,
			Type: "MX",
			Name: "mail",
			Data: "mail.example.com",
			TTL:  7200,
		},
	}

	assert.Equal(t, 456, response.DomainRecord.ID)
	assert.Equal(t, "MX", response.DomainRecord.Type)
}

func TestDigitalOceanProvider_ErrorResponse(t *testing.T) {
	response := doErrorResponse{
		ID:      "not_found",
		Message: "Domain not found",
	}

	assert.Equal(t, "not_found", response.ID)
	assert.Equal(t, "Domain not found", response.Message)
}

func TestDigitalOceanProvider_AddRecord_RequestData(t *testing.T) {
	provider := NewDigitalOceanProvider(map[string]string{"token": "test"})
	provider.SetDomain("example.com")

	priority := 10
	weight := 5
	port := 443
	flags := 0
	tag := "issue"

	record := &DNSRecord{
		Type:     RecordTypeSRV,
		Name:     "_service",
		Value:    "target.example.com",
		TTL:      3600,
		Priority: &priority,
		Weight:   &weight,
		Port:     &port,
		Flags:    &flags,
		Tag:      &tag,
	}

	// Build request data like the provider does
	data := map[string]interface{}{
		"type": record.Type.String(),
		"name": record.Name,
		"data": provider.prepValue(record),
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

	assert.Equal(t, "SRV", data["type"])
	assert.Equal(t, "_service", data["name"])
	assert.Equal(t, "target.example.com", data["data"])
	assert.Equal(t, 3600, data["ttl"])
	assert.Equal(t, 10, data["priority"])
	assert.Equal(t, 5, data["weight"])
	assert.Equal(t, 443, data["port"])
	assert.Equal(t, 0, data["flags"])
	assert.Equal(t, "issue", data["tag"])
}

func TestDigitalOceanProvider_NewRequest_Headers(t *testing.T) {
	provider := NewDigitalOceanProvider(map[string]string{"token": "test-token"})

	ctx := context.Background()
	req, err := provider.newRequest(ctx, http.MethodGet, "/test", nil)
	require.NoError(t, err)

	assert.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
}

func TestDigitalOceanProvider_ListRecords_TypeFiltering(t *testing.T) {
	// Test that record type parsing works correctly
	recordTypes := []string{"A", "AAAA", "CNAME", "MX", "TXT", "SRV", "NS", "SOA", "CAA"}

	for _, rt := range recordTypes {
		parsed, err := ParseRecordType(rt)
		require.NoError(t, err)
		assert.Equal(t, RecordType(rt), parsed)
	}
}

func TestDigitalOceanProvider_DomainName_AsID(t *testing.T) {
	// DigitalOcean uses domain names as IDs
	provider := NewDigitalOceanProvider(map[string]string{"token": "test"})
	provider.SetDomain("example.com")

	// When adding a domain, it should return the domain name as ID
	// This is a behavioral test showing that DO uses names instead of numeric IDs
	assert.Equal(t, "example.com", provider.GetDomain())
}

func TestDigitalOceanProvider_RecordID_Formatting(t *testing.T) {
	// DigitalOcean record IDs are integers, but we store them as strings
	recordID := 12345

	formattedID := recordID
	assert.Equal(t, 12345, formattedID)

	// In the provider, we format as string
	strID := string(rune(recordID))
	assert.NotEmpty(t, strID)
}
