package dns

import (
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/dns/enums"
)

func TestRecordType_String(t *testing.T) {
	tests := []struct {
		rt       enums.RecordType
		expected string
	}{
		{enums.RecordTypeA, "A"},
		{enums.RecordTypeAAAA, "AAAA"},
		{enums.RecordTypeCNAME, "CNAME"},
		{enums.RecordTypeMX, "MX"},
		{enums.RecordTypeNS, "NS"},
		{enums.RecordTypeSRV, "SRV"},
		{enums.RecordTypeTXT, "TXT"},
		{enums.RecordTypeSOA, "SOA"},
		{enums.RecordTypeCAA, "CAA"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.String())
		})
	}
}

func TestRecordType_IsValid(t *testing.T) {
	tests := []struct {
		rt       enums.RecordType
		expected bool
	}{
		{enums.RecordTypeA, true},
		{enums.RecordTypeAAAA, true},
		{enums.RecordTypeCNAME, true},
		{enums.RecordTypeMX, true},
		{enums.RecordTypeNS, true},
		{enums.RecordTypeSRV, true},
		{enums.RecordTypeTXT, true},
		{enums.RecordTypeSOA, true},
		{enums.RecordTypeCAA, true},
		{enums.RecordType("INVALID"), false},
		{enums.RecordType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.IsValid())
		})
	}
}

func TestRecordType_SupportsProxy(t *testing.T) {
	tests := []struct {
		rt       enums.RecordType
		expected bool
	}{
		{enums.RecordTypeA, true},
		{enums.RecordTypeAAAA, true},
		{enums.RecordTypeCNAME, true},
		{enums.RecordTypeMX, false},
		{enums.RecordTypeNS, false},
		{enums.RecordTypeSRV, false},
		{enums.RecordTypeTXT, false},
		{enums.RecordTypeSOA, false},
		{enums.RecordTypeCAA, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.SupportsProxy())
		})
	}
}

func TestRecordType_RequiresPriority(t *testing.T) {
	tests := []struct {
		rt       enums.RecordType
		expected bool
	}{
		{enums.RecordTypeA, false},
		{enums.RecordTypeAAAA, false},
		{enums.RecordTypeCNAME, false},
		{enums.RecordTypeMX, true},
		{enums.RecordTypeNS, false},
		{enums.RecordTypeSRV, true},
		{enums.RecordTypeTXT, false},
		{enums.RecordTypeSOA, false},
		{enums.RecordTypeCAA, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.RequiresPriority())
		})
	}
}

func TestAllRecordTypes(t *testing.T) {
	types := enums.AllRecordTypes()

	assert.Len(t, types, 9)
	assert.Contains(t, types, enums.RecordTypeA)
	assert.Contains(t, types, enums.RecordTypeAAAA)
	assert.Contains(t, types, enums.RecordTypeCNAME)
	assert.Contains(t, types, enums.RecordTypeMX)
	assert.Contains(t, types, enums.RecordTypeNS)
	assert.Contains(t, types, enums.RecordTypeSRV)
	assert.Contains(t, types, enums.RecordTypeTXT)
	assert.Contains(t, types, enums.RecordTypeSOA)
	assert.Contains(t, types, enums.RecordTypeCAA)
}

func TestParseRecordType(t *testing.T) {
	tests := []struct {
		input    string
		expected enums.RecordType
		wantErr  bool
	}{
		{"A", enums.RecordTypeA, false},
		{"AAAA", enums.RecordTypeAAAA, false},
		{"CNAME", enums.RecordTypeCNAME, false},
		{"MX", enums.RecordTypeMX, false},
		{"NS", enums.RecordTypeNS, false},
		{"SRV", enums.RecordTypeSRV, false},
		{"TXT", enums.RecordTypeTXT, false},
		{"SOA", enums.RecordTypeSOA, false},
		{"CAA", enums.RecordTypeCAA, false},
		{"INVALID", enums.RecordType(""), true},
		{"", enums.RecordType(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			rt, err := enums.ParseRecordType(tt.input)

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
		p        enums.DnsProvider
		expected string
	}{
		{enums.DnsProviderCloudflare, "cloudflare"},
		{enums.DnsProviderDigitalOcean, "digitalocean"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.p.String())
		})
	}
}

func TestDnsProvider_Label(t *testing.T) {
	tests := []struct {
		p        enums.DnsProvider
		expected string
	}{
		{enums.DnsProviderCloudflare, "Cloudflare"},
		{enums.DnsProviderDigitalOcean, "DigitalOcean"},
		{enums.DnsProvider("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.p.Label())
		})
	}
}

func TestDnsProvider_IsValid(t *testing.T) {
	tests := []struct {
		p        enums.DnsProvider
		expected bool
	}{
		{enums.DnsProviderCloudflare, true},
		{enums.DnsProviderDigitalOcean, true},
		{enums.DnsProvider("invalid"), false},
		{enums.DnsProvider(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.p), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.p.IsValid())
		})
	}
}

func TestDnsProvider_Value(t *testing.T) {
	p := enums.DnsProviderCloudflare
	val, err := p.Value()

	require.NoError(t, err)
	assert.Equal(t, driver.Value("cloudflare"), val)
}

func TestDnsProvider_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected enums.DnsProvider
		wantErr  bool
	}{
		{"string cloudflare", "cloudflare", enums.DnsProviderCloudflare, false},
		{"string digitalocean", "digitalocean", enums.DnsProviderDigitalOcean, false},
		{"bytes", []byte("cloudflare"), enums.DnsProviderCloudflare, false},
		{"nil", nil, enums.DnsProvider(""), false},
		{"invalid type", 123, enums.DnsProvider(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p enums.DnsProvider
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
	providers := enums.AllDnsProviders()

	assert.Len(t, providers, 2)
	assert.Contains(t, providers, enums.DnsProviderCloudflare)
	assert.Contains(t, providers, enums.DnsProviderDigitalOcean)
}

func TestParseDnsProvider(t *testing.T) {
	tests := []struct {
		input    string
		expected enums.DnsProvider
		wantErr  bool
	}{
		{"cloudflare", enums.DnsProviderCloudflare, false},
		{"digitalocean", enums.DnsProviderDigitalOcean, false},
		{"invalid", enums.DnsProvider(""), true},
		{"", enums.DnsProvider(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p, err := enums.ParseDnsProvider(tt.input)

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
		s        enums.SyncStatus
		expected string
	}{
		{enums.SyncStatusPending, "pending"},
		{enums.SyncStatusSyncing, "syncing"},
		{enums.SyncStatusCompleted, "completed"},
		{enums.SyncStatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.s.String())
		})
	}
}

func TestSyncStatus_IsValid(t *testing.T) {
	tests := []struct {
		s        enums.SyncStatus
		expected bool
	}{
		{enums.SyncStatusPending, true},
		{enums.SyncStatusSyncing, true},
		{enums.SyncStatusCompleted, true},
		{enums.SyncStatusFailed, true},
		{enums.SyncStatus("invalid"), false},
		{enums.SyncStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.s), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.s.IsValid())
		})
	}
}

func TestSyncStatus_Value(t *testing.T) {
	s := enums.SyncStatusCompleted
	val, err := s.Value()

	require.NoError(t, err)
	assert.Equal(t, driver.Value("completed"), val)
}

func TestSyncStatus_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected enums.SyncStatus
		wantErr  bool
	}{
		{"string pending", "pending", enums.SyncStatusPending, false},
		{"string completed", "completed", enums.SyncStatusCompleted, false},
		{"bytes", []byte("syncing"), enums.SyncStatusSyncing, false},
		{"nil", nil, enums.SyncStatus(""), false},
		{"invalid type", 123, enums.SyncStatus(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s enums.SyncStatus
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
	p := enums.DnsProviderCloudflare
	pt := p.ToProviderType()
	assert.Equal(t, "cloudflare", pt.String())

	p = enums.DnsProviderDigitalOcean
	pt = p.ToProviderType()
	assert.Equal(t, "digitalocean", pt.String())
}
