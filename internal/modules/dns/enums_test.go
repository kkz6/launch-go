package dns

import (
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordType_String(t *testing.T) {
	tests := []struct {
		rt       RecordType
		expected string
	}{
		{RecordTypeA, "A"},
		{RecordTypeAAAA, "AAAA"},
		{RecordTypeCNAME, "CNAME"},
		{RecordTypeMX, "MX"},
		{RecordTypeNS, "NS"},
		{RecordTypeSRV, "SRV"},
		{RecordTypeTXT, "TXT"},
		{RecordTypeSOA, "SOA"},
		{RecordTypeCAA, "CAA"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.String())
		})
	}
}

func TestRecordType_IsValid(t *testing.T) {
	tests := []struct {
		rt       RecordType
		expected bool
	}{
		{RecordTypeA, true},
		{RecordTypeAAAA, true},
		{RecordTypeCNAME, true},
		{RecordTypeMX, true},
		{RecordTypeNS, true},
		{RecordTypeSRV, true},
		{RecordTypeTXT, true},
		{RecordTypeSOA, true},
		{RecordTypeCAA, true},
		{RecordType("INVALID"), false},
		{RecordType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.IsValid())
		})
	}
}

func TestRecordType_SupportsProxy(t *testing.T) {
	tests := []struct {
		rt       RecordType
		expected bool
	}{
		{RecordTypeA, true},
		{RecordTypeAAAA, true},
		{RecordTypeCNAME, true},
		{RecordTypeMX, false},
		{RecordTypeNS, false},
		{RecordTypeSRV, false},
		{RecordTypeTXT, false},
		{RecordTypeSOA, false},
		{RecordTypeCAA, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.SupportsProxy())
		})
	}
}

func TestRecordType_RequiresPriority(t *testing.T) {
	tests := []struct {
		rt       RecordType
		expected bool
	}{
		{RecordTypeA, false},
		{RecordTypeAAAA, false},
		{RecordTypeCNAME, false},
		{RecordTypeMX, true},
		{RecordTypeNS, false},
		{RecordTypeSRV, true},
		{RecordTypeTXT, false},
		{RecordTypeSOA, false},
		{RecordTypeCAA, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.RequiresPriority())
		})
	}
}

func TestAllRecordTypes(t *testing.T) {
	types := AllRecordTypes()

	assert.Len(t, types, 9)
	assert.Contains(t, types, RecordTypeA)
	assert.Contains(t, types, RecordTypeAAAA)
	assert.Contains(t, types, RecordTypeCNAME)
	assert.Contains(t, types, RecordTypeMX)
	assert.Contains(t, types, RecordTypeNS)
	assert.Contains(t, types, RecordTypeSRV)
	assert.Contains(t, types, RecordTypeTXT)
	assert.Contains(t, types, RecordTypeSOA)
	assert.Contains(t, types, RecordTypeCAA)
}

func TestParseRecordType(t *testing.T) {
	tests := []struct {
		input    string
		expected RecordType
		wantErr  bool
	}{
		{"A", RecordTypeA, false},
		{"AAAA", RecordTypeAAAA, false},
		{"CNAME", RecordTypeCNAME, false},
		{"MX", RecordTypeMX, false},
		{"NS", RecordTypeNS, false},
		{"SRV", RecordTypeSRV, false},
		{"TXT", RecordTypeTXT, false},
		{"SOA", RecordTypeSOA, false},
		{"CAA", RecordTypeCAA, false},
		{"INVALID", RecordType(""), true},
		{"", RecordType(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			rt, err := ParseRecordType(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, rt)
			}
		})
	}
}

func TestDnsProvider_String(t *testing.T) {
	tests := []struct {
		p        DnsProvider
		expected string
	}{
		{DnsProviderCloudflare, "cloudflare"},
		{DnsProviderDigitalOcean, "digitalocean"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.p.String())
		})
	}
}

func TestDnsProvider_Label(t *testing.T) {
	tests := []struct {
		p        DnsProvider
		expected string
	}{
		{DnsProviderCloudflare, "Cloudflare"},
		{DnsProviderDigitalOcean, "DigitalOcean"},
		{DnsProvider("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.p.Label())
		})
	}
}

func TestDnsProvider_IsValid(t *testing.T) {
	tests := []struct {
		p        DnsProvider
		expected bool
	}{
		{DnsProviderCloudflare, true},
		{DnsProviderDigitalOcean, true},
		{DnsProvider("invalid"), false},
		{DnsProvider(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.p), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.p.IsValid())
		})
	}
}

func TestDnsProvider_Value(t *testing.T) {
	p := DnsProviderCloudflare
	val, err := p.Value()

	require.NoError(t, err)
	assert.Equal(t, driver.Value("cloudflare"), val)
}

func TestDnsProvider_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected DnsProvider
		wantErr  bool
	}{
		{"string cloudflare", "cloudflare", DnsProviderCloudflare, false},
		{"string digitalocean", "digitalocean", DnsProviderDigitalOcean, false},
		{"bytes", []byte("cloudflare"), DnsProviderCloudflare, false},
		{"nil", nil, DnsProvider(""), false},
		{"invalid type", 123, DnsProvider(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p DnsProvider
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

func TestAllDnsProviders(t *testing.T) {
	providers := AllDnsProviders()

	assert.Len(t, providers, 2)
	assert.Contains(t, providers, DnsProviderCloudflare)
	assert.Contains(t, providers, DnsProviderDigitalOcean)
}

func TestParseDnsProvider(t *testing.T) {
	tests := []struct {
		input    string
		expected DnsProvider
		wantErr  bool
	}{
		{"cloudflare", DnsProviderCloudflare, false},
		{"digitalocean", DnsProviderDigitalOcean, false},
		{"invalid", DnsProvider(""), true},
		{"", DnsProvider(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p, err := ParseDnsProvider(tt.input)

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
		s        SyncStatus
		expected string
	}{
		{SyncStatusPending, "pending"},
		{SyncStatusSyncing, "syncing"},
		{SyncStatusCompleted, "completed"},
		{SyncStatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.s.String())
		})
	}
}

func TestSyncStatus_IsValid(t *testing.T) {
	tests := []struct {
		s        SyncStatus
		expected bool
	}{
		{SyncStatusPending, true},
		{SyncStatusSyncing, true},
		{SyncStatusCompleted, true},
		{SyncStatusFailed, true},
		{SyncStatus("invalid"), false},
		{SyncStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.s), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.s.IsValid())
		})
	}
}

func TestSyncStatus_Value(t *testing.T) {
	s := SyncStatusCompleted
	val, err := s.Value()

	require.NoError(t, err)
	assert.Equal(t, driver.Value("completed"), val)
}

func TestSyncStatus_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected SyncStatus
		wantErr  bool
	}{
		{"string pending", "pending", SyncStatusPending, false},
		{"string completed", "completed", SyncStatusCompleted, false},
		{"bytes", []byte("syncing"), SyncStatusSyncing, false},
		{"nil", nil, SyncStatus(""), false},
		{"invalid type", 123, SyncStatus(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s SyncStatus
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

func TestDnsProvider_ToProviderType(t *testing.T) {
	p := DnsProviderCloudflare
	pt := p.ToProviderType()
	assert.Equal(t, "cloudflare", pt.String())

	p = DnsProviderDigitalOcean
	pt = p.ToProviderType()
	assert.Equal(t, "digitalocean", pt.String())
}
