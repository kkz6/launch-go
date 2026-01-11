package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseProvider_SetDomain(t *testing.T) {
	p := &BaseProvider{}
	p.SetDomain("example.com")

	assert.Equal(t, "example.com", p.GetDomain())
}

func TestBaseProvider_SetCredentials(t *testing.T) {
	p := &BaseProvider{}
	creds := map[string]string{"token": "test-token"}
	p.SetCredentials(creds)

	assert.Equal(t, "test-token", p.GetCredentials()["token"])
}

func TestBaseProvider_GetToken(t *testing.T) {
	p := &BaseProvider{}
	creds := map[string]string{"token": "test-token"}
	p.SetCredentials(creds)

	assert.Equal(t, "test-token", p.GetToken())
}

func TestBaseProvider_GetToken_NilCredentials(t *testing.T) {
	p := &BaseProvider{}

	assert.Empty(t, p.GetToken())
}

func TestWithTrailingDot(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com."},
		{"example.com.", "example.com."},
		{"@", "@"},
		{"www.example.com", "www.example.com."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := WithTrailingDot(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewProvider_Cloudflare(t *testing.T) {
	creds := map[string]string{"token": "test-token"}
	additionalData := map[string]interface{}{"account_id": "acc123"}

	provider, err := NewProvider(DnsProviderTypeCloudflare, creds, additionalData)
	require.NoError(t, err)

	assert.NotNil(t, provider)
	assert.Equal(t, "Cloudflare", provider.Name())
}

func TestNewProvider_DigitalOcean(t *testing.T) {
	creds := map[string]string{"token": "test-token"}

	provider, err := NewProvider(DnsProviderTypeDigitalOcean, creds, nil)
	require.NoError(t, err)

	assert.NotNil(t, provider)
	assert.Equal(t, "DigitalOcean", provider.Name())
}

func TestNewProvider_Invalid(t *testing.T) {
	creds := map[string]string{"token": "test-token"}

	_, err := NewProvider(DnsProviderType("invalid"), creds, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestProviderError_Error(t *testing.T) {
	err := &ProviderError{
		Provider: "Cloudflare",
		Code:     403,
		Message:  "Access denied",
		Err:      nil,
	}

	assert.Equal(t, "Cloudflare provider error (code 403): Access denied", err.Error())
}

func TestProviderError_Error_WithWrapped(t *testing.T) {
	wrappedErr := assert.AnError

	err := &ProviderError{
		Provider: "Cloudflare",
		Code:     500,
		Message:  "Internal error",
		Err:      wrappedErr,
	}

	expected := "Cloudflare provider error (code 500): Internal error: " + wrappedErr.Error()
	assert.Equal(t, expected, err.Error())
}

func TestProviderError_Unwrap(t *testing.T) {
	wrappedErr := assert.AnError

	err := &ProviderError{
		Provider: "Cloudflare",
		Code:     500,
		Message:  "Internal error",
		Err:      wrappedErr,
	}

	assert.Equal(t, wrappedErr, err.Unwrap())
}

func TestNewProviderError(t *testing.T) {
	err := NewProviderError("DigitalOcean", 404, "Not found", nil)

	assert.Equal(t, "DigitalOcean", err.Provider)
	assert.Equal(t, 404, err.Code)
	assert.Equal(t, "Not found", err.Message)
	assert.Nil(t, err.Err)
}

func TestRecordType_String(t *testing.T) {
	assert.Equal(t, "A", RecordTypeA.String())
	assert.Equal(t, "AAAA", RecordTypeAAAA.String())
	assert.Equal(t, "CNAME", RecordTypeCNAME.String())
	assert.Equal(t, "MX", RecordTypeMX.String())
	assert.Equal(t, "NS", RecordTypeNS.String())
	assert.Equal(t, "TXT", RecordTypeTXT.String())
	assert.Equal(t, "SRV", RecordTypeSRV.String())
	assert.Equal(t, "SOA", RecordTypeSOA.String())
	assert.Equal(t, "CAA", RecordTypeCAA.String())
}

func TestRecordType_IsValid(t *testing.T) {
	assert.True(t, RecordTypeA.IsValid())
	assert.True(t, RecordTypeAAAA.IsValid())
	assert.True(t, RecordTypeCNAME.IsValid())
	assert.True(t, RecordTypeMX.IsValid())
	assert.True(t, RecordTypeNS.IsValid())
	assert.True(t, RecordTypeSRV.IsValid())
	assert.True(t, RecordTypeTXT.IsValid())
	assert.True(t, RecordTypeSOA.IsValid())
	assert.True(t, RecordTypeCAA.IsValid())
	assert.False(t, RecordType("INVALID").IsValid())
}

func TestRecordType_SupportsProxy(t *testing.T) {
	assert.True(t, RecordTypeA.SupportsProxy())
	assert.True(t, RecordTypeAAAA.SupportsProxy())
	assert.True(t, RecordTypeCNAME.SupportsProxy())
	assert.False(t, RecordTypeMX.SupportsProxy())
	assert.False(t, RecordTypeNS.SupportsProxy())
	assert.False(t, RecordTypeTXT.SupportsProxy())
}

func TestRecordType_RequiresPriority(t *testing.T) {
	assert.True(t, RecordTypeMX.RequiresPriority())
	assert.True(t, RecordTypeSRV.RequiresPriority())
	assert.False(t, RecordTypeA.RequiresPriority())
	assert.False(t, RecordTypeCNAME.RequiresPriority())
}

func TestParseRecordType(t *testing.T) {
	rt, err := ParseRecordType("A")
	require.NoError(t, err)
	assert.Equal(t, RecordTypeA, rt)

	rt, err = ParseRecordType("CNAME")
	require.NoError(t, err)
	assert.Equal(t, RecordTypeCNAME, rt)

	_, err = ParseRecordType("INVALID")
	assert.Error(t, err)
}

func TestAllRecordTypes(t *testing.T) {
	types := AllRecordTypes()
	assert.Len(t, types, 9)
	assert.Contains(t, types, RecordTypeA)
	assert.Contains(t, types, RecordTypeAAAA)
	assert.Contains(t, types, RecordTypeCNAME)
}

func TestDnsProviderType_String(t *testing.T) {
	assert.Equal(t, "cloudflare", DnsProviderTypeCloudflare.String())
	assert.Equal(t, "digitalocean", DnsProviderTypeDigitalOcean.String())
}

func TestDnsProviderType_Label(t *testing.T) {
	assert.Equal(t, "Cloudflare", DnsProviderTypeCloudflare.Label())
	assert.Equal(t, "DigitalOcean", DnsProviderTypeDigitalOcean.Label())
	assert.Equal(t, "unknown", DnsProviderType("unknown").Label())
}

func TestDnsProviderType_IsValid(t *testing.T) {
	assert.True(t, DnsProviderTypeCloudflare.IsValid())
	assert.True(t, DnsProviderTypeDigitalOcean.IsValid())
	assert.False(t, DnsProviderType("invalid").IsValid())
}

func TestAllDnsProviderTypes(t *testing.T) {
	types := AllDnsProviderTypes()
	assert.Len(t, types, 2)
	assert.Contains(t, types, DnsProviderTypeCloudflare)
	assert.Contains(t, types, DnsProviderTypeDigitalOcean)
}

func TestParseDnsProviderType(t *testing.T) {
	pt, err := ParseDnsProviderType("cloudflare")
	require.NoError(t, err)
	assert.Equal(t, DnsProviderTypeCloudflare, pt)

	pt, err = ParseDnsProviderType("digitalocean")
	require.NoError(t, err)
	assert.Equal(t, DnsProviderTypeDigitalOcean, pt)

	_, err = ParseDnsProviderType("invalid")
	assert.Error(t, err)
}

func TestDnsRecord_IsEditable(t *testing.T) {
	record := &DnsRecord{Type: RecordTypeA}
	assert.True(t, record.IsEditable())

	nsRecord := &DnsRecord{Type: RecordTypeNS}
	assert.False(t, nsRecord.IsEditable())

	soaRecord := &DnsRecord{Type: RecordTypeSOA}
	assert.False(t, soaRecord.IsEditable())
}

func TestDnsRecord_IsDeletable(t *testing.T) {
	record := &DnsRecord{Type: RecordTypeA}
	assert.True(t, record.IsDeletable())

	nsRecord := &DnsRecord{Type: RecordTypeNS}
	assert.False(t, nsRecord.IsDeletable())

	soaRecord := &DnsRecord{Type: RecordTypeSOA}
	assert.False(t, soaRecord.IsDeletable())
}

func TestProviderRecord_Fields(t *testing.T) {
	priority := 10
	tag := "issue"
	weight := 100
	port := 443
	flags := 128
	comment := "test"
	proxied := true

	record := ProviderRecord{
		ID:       "rec123",
		Type:     RecordTypeA,
		Name:     "www",
		Value:    "1.2.3.4",
		TTL:      3600,
		Priority: &priority,
		Tag:      &tag,
		Weight:   &weight,
		Port:     &port,
		Flags:    &flags,
		Comment:  &comment,
		Proxied:  &proxied,
	}

	assert.Equal(t, "rec123", record.ID)
	assert.Equal(t, RecordTypeA, record.Type)
	assert.Equal(t, "www", record.Name)
	assert.Equal(t, "1.2.3.4", record.Value)
	assert.Equal(t, 3600, record.TTL)
	assert.Equal(t, 10, *record.Priority)
	assert.Equal(t, "issue", *record.Tag)
	assert.Equal(t, 100, *record.Weight)
	assert.Equal(t, 443, *record.Port)
	assert.Equal(t, 128, *record.Flags)
	assert.Equal(t, "test", *record.Comment)
	assert.True(t, *record.Proxied)
}
