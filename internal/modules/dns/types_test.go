package dns

import (
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dnstypes "github.com/kkz6/launch-go/internal/modules/dns/types"
)

func TestRecordType_String(t *testing.T) {
	tests := []struct {
		rt       dnstypes.RecordType
		expected string
	}{
		{dnstypes.RecordTypeA, "A"},
		{dnstypes.RecordTypeAAAA, "AAAA"},
		{dnstypes.RecordTypeCNAME, "CNAME"},
		{dnstypes.RecordTypeMX, "MX"},
		{dnstypes.RecordTypeNS, "NS"},
		{dnstypes.RecordTypeSRV, "SRV"},
		{dnstypes.RecordTypeTXT, "TXT"},
		{dnstypes.RecordTypeSOA, "SOA"},
		{dnstypes.RecordTypeCAA, "CAA"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.String())
		})
	}
}

func TestRecordType_IsValid(t *testing.T) {
	tests := []struct {
		rt       dnstypes.RecordType
		expected bool
	}{
		{dnstypes.RecordTypeA, true},
		{dnstypes.RecordTypeAAAA, true},
		{dnstypes.RecordTypeCNAME, true},
		{dnstypes.RecordTypeMX, true},
		{dnstypes.RecordTypeNS, true},
		{dnstypes.RecordTypeSRV, true},
		{dnstypes.RecordTypeTXT, true},
		{dnstypes.RecordTypeSOA, true},
		{dnstypes.RecordTypeCAA, true},
		{dnstypes.RecordType("INVALID"), false},
		{dnstypes.RecordType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.IsValid())
		})
	}
}

func TestRecordType_SupportsProxy(t *testing.T) {
	tests := []struct {
		rt       dnstypes.RecordType
		expected bool
	}{
		{dnstypes.RecordTypeA, true},
		{dnstypes.RecordTypeAAAA, true},
		{dnstypes.RecordTypeCNAME, true},
		{dnstypes.RecordTypeMX, false},
		{dnstypes.RecordTypeNS, false},
		{dnstypes.RecordTypeSRV, false},
		{dnstypes.RecordTypeTXT, false},
		{dnstypes.RecordTypeSOA, false},
		{dnstypes.RecordTypeCAA, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.SupportsProxy())
		})
	}
}

func TestRecordType_RequiresPriority(t *testing.T) {
	tests := []struct {
		rt       dnstypes.RecordType
		expected bool
	}{
		{dnstypes.RecordTypeA, false},
		{dnstypes.RecordTypeAAAA, false},
		{dnstypes.RecordTypeCNAME, false},
		{dnstypes.RecordTypeMX, true},
		{dnstypes.RecordTypeNS, false},
		{dnstypes.RecordTypeSRV, true},
		{dnstypes.RecordTypeTXT, false},
		{dnstypes.RecordTypeSOA, false},
		{dnstypes.RecordTypeCAA, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.RequiresPriority())
		})
	}
}

func TestAllRecordTypes(t *testing.T) {
	types := dnstypes.AllRecordTypes()

	assert.Len(t, types, 9)
	assert.Contains(t, types, dnstypes.RecordTypeA)
	assert.Contains(t, types, dnstypes.RecordTypeAAAA)
	assert.Contains(t, types, dnstypes.RecordTypeCNAME)
	assert.Contains(t, types, dnstypes.RecordTypeMX)
	assert.Contains(t, types, dnstypes.RecordTypeNS)
	assert.Contains(t, types, dnstypes.RecordTypeSRV)
	assert.Contains(t, types, dnstypes.RecordTypeTXT)
	assert.Contains(t, types, dnstypes.RecordTypeSOA)
	assert.Contains(t, types, dnstypes.RecordTypeCAA)
}

func TestParseRecordType(t *testing.T) {
	tests := []struct {
		input    string
		expected dnstypes.RecordType
		wantErr  bool
	}{
		{"A", dnstypes.RecordTypeA, false},
		{"AAAA", dnstypes.RecordTypeAAAA, false},
		{"CNAME", dnstypes.RecordTypeCNAME, false},
		{"MX", dnstypes.RecordTypeMX, false},
		{"NS", dnstypes.RecordTypeNS, false},
		{"SRV", dnstypes.RecordTypeSRV, false},
		{"TXT", dnstypes.RecordTypeTXT, false},
		{"SOA", dnstypes.RecordTypeSOA, false},
		{"CAA", dnstypes.RecordTypeCAA, false},
		{"INVALID", dnstypes.RecordType(""), true},
		{"", dnstypes.RecordType(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			rt, err := dnstypes.ParseRecordType(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, rt)
			}
		})
	}
}

func TestDNSProvider_String(t *testing.T) {
	tests := []struct {
		p        dnstypes.DNSProvider
		expected string
	}{
		{dnstypes.DNSProviderCloudflare, "cloudflare"},
		{dnstypes.DNSProviderDigitalOcean, "digitalocean"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.p.String())
		})
	}
}

func TestDNSProvider_Label(t *testing.T) {
	tests := []struct {
		p        dnstypes.DNSProvider
		expected string
	}{
		{dnstypes.DNSProviderCloudflare, "Cloudflare"},
		{dnstypes.DNSProviderDigitalOcean, "DigitalOcean"},
		{dnstypes.DNSProvider("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.p.Label())
		})
	}
}

func TestDNSProvider_IsValid(t *testing.T) {
	tests := []struct {
		p        dnstypes.DNSProvider
		expected bool
	}{
		{dnstypes.DNSProviderCloudflare, true},
		{dnstypes.DNSProviderDigitalOcean, true},
		{dnstypes.DNSProvider("invalid"), false},
		{dnstypes.DNSProvider(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.p), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.p.IsValid())
		})
	}
}

func TestDNSProvider_Value(t *testing.T) {
	p := dnstypes.DNSProviderCloudflare
	val, err := p.Value()

	require.NoError(t, err)
	assert.Equal(t, driver.Value("cloudflare"), val)
}

func TestDNSProvider_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected dnstypes.DNSProvider
		wantErr  bool
	}{
		{"string cloudflare", "cloudflare", dnstypes.DNSProviderCloudflare, false},
		{"string digitalocean", "digitalocean", dnstypes.DNSProviderDigitalOcean, false},
		{"bytes", []byte("cloudflare"), dnstypes.DNSProviderCloudflare, false},
		{"nil", nil, dnstypes.DNSProvider(""), false},
		{"invalid type", 123, dnstypes.DNSProvider(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p dnstypes.DNSProvider
			err := p.Scan(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, p)
			}
		})
	}
}

func TestAllDNSProviders(t *testing.T) {
	providers := dnstypes.AllDNSProviders()

	assert.Len(t, providers, 2)
	assert.Contains(t, providers, dnstypes.DNSProviderCloudflare)
	assert.Contains(t, providers, dnstypes.DNSProviderDigitalOcean)
}

func TestParseDNSProvider(t *testing.T) {
	tests := []struct {
		input    string
		expected dnstypes.DNSProvider
		wantErr  bool
	}{
		{"cloudflare", dnstypes.DNSProviderCloudflare, false},
		{"digitalocean", dnstypes.DNSProviderDigitalOcean, false},
		{"invalid", dnstypes.DNSProvider(""), true},
		{"", dnstypes.DNSProvider(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p, err := dnstypes.ParseDNSProvider(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, p)
			}
		})
	}
}

func TestSyncStatus_String(t *testing.T) {
	tests := []struct {
		s        dnstypes.SyncStatus
		expected string
	}{
		{dnstypes.SyncStatusPending, "pending"},
		{dnstypes.SyncStatusSyncing, "syncing"},
		{dnstypes.SyncStatusCompleted, "completed"},
		{dnstypes.SyncStatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.s.String())
		})
	}
}

func TestSyncStatus_IsValid(t *testing.T) {
	tests := []struct {
		s        dnstypes.SyncStatus
		expected bool
	}{
		{dnstypes.SyncStatusPending, true},
		{dnstypes.SyncStatusSyncing, true},
		{dnstypes.SyncStatusCompleted, true},
		{dnstypes.SyncStatusFailed, true},
		{dnstypes.SyncStatus("invalid"), false},
		{dnstypes.SyncStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.s), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.s.IsValid())
		})
	}
}

func TestSyncStatus_Value(t *testing.T) {
	s := dnstypes.SyncStatusCompleted
	val, err := s.Value()

	require.NoError(t, err)
	assert.Equal(t, driver.Value("completed"), val)
}

func TestSyncStatus_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected dnstypes.SyncStatus
		wantErr  bool
	}{
		{"string pending", "pending", dnstypes.SyncStatusPending, false},
		{"string completed", "completed", dnstypes.SyncStatusCompleted, false},
		{"bytes", []byte("syncing"), dnstypes.SyncStatusSyncing, false},
		{"nil", nil, dnstypes.SyncStatus(""), false},
		{"invalid type", 123, dnstypes.SyncStatus(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s dnstypes.SyncStatus
			err := s.Scan(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, s)
			}
		})
	}
}

func TestDNSProvider_ToProviderType(t *testing.T) {
	p := dnstypes.DNSProviderCloudflare
	pt := p.ToProviderType()
	assert.Equal(t, "cloudflare", pt.String())

	p = dnstypes.DNSProviderDigitalOcean
	pt = p.ToProviderType()
	assert.Equal(t, "digitalocean", pt.String())
}
