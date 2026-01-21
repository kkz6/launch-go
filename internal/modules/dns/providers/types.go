package providers

import (
	"fmt"
)

// RecordType represents the type of DNS record (for provider operations)
type RecordType string

const (
	RecordTypeA     RecordType = "A"
	RecordTypeAAAA  RecordType = "AAAA"
	RecordTypeCNAME RecordType = "CNAME"
	RecordTypeMX    RecordType = "MX"
	RecordTypeNS    RecordType = "NS"
	RecordTypeSRV   RecordType = "SRV"
	RecordTypeTXT   RecordType = "TXT"
	RecordTypeSOA   RecordType = "SOA"
	RecordTypeCAA   RecordType = "CAA"
)

// String returns the string representation of RecordType
func (r RecordType) String() string {
	return string(r)
}

// IsValid checks if the RecordType is valid
func (r RecordType) IsValid() bool {
	switch r {
	case RecordTypeA, RecordTypeAAAA, RecordTypeCNAME, RecordTypeMX,
		RecordTypeNS, RecordTypeSRV, RecordTypeTXT, RecordTypeSOA, RecordTypeCAA:
		return true
	}
	return false
}

// SupportsProxy returns true if this record type supports Cloudflare proxy
func (r RecordType) SupportsProxy() bool {
	switch r {
	case RecordTypeA, RecordTypeAAAA, RecordTypeCNAME:
		return true
	}
	return false
}

// RequiresPriority returns true if this record type requires a priority field
func (r RecordType) RequiresPriority() bool {
	return r == RecordTypeMX || r == RecordTypeSRV
}

// AllRecordTypes returns all valid record types
func AllRecordTypes() []RecordType {
	return []RecordType{
		RecordTypeA,
		RecordTypeAAAA,
		RecordTypeCNAME,
		RecordTypeMX,
		RecordTypeNS,
		RecordTypeSRV,
		RecordTypeTXT,
		RecordTypeSOA,
		RecordTypeCAA,
	}
}

// ParseRecordType parses a string into a RecordType
func ParseRecordType(s string) (RecordType, error) {
	rt := RecordType(s)
	if !rt.IsValid() {
		return "", fmt.Errorf("invalid record type: %s", s)
	}
	return rt, nil
}

// DnsProviderType represents a DNS provider type
type DnsProviderType string

const (
	DnsProviderTypeCloudflare   DnsProviderType = "cloudflare"
	DnsProviderTypeDigitalOcean DnsProviderType = "digitalocean"
)

// String returns the string representation of DnsProviderType
func (p DnsProviderType) String() string {
	return string(p)
}

// Label returns a human-readable label for the provider
func (p DnsProviderType) Label() string {
	switch p {
	case DnsProviderTypeCloudflare:
		return "Cloudflare"
	case DnsProviderTypeDigitalOcean:
		return "DigitalOcean"
	default:
		return string(p)
	}
}

// IsValid checks if the DnsProviderType is valid
func (p DnsProviderType) IsValid() bool {
	switch p {
	case DnsProviderTypeCloudflare, DnsProviderTypeDigitalOcean:
		return true
	}
	return false
}

// AllDnsProviderTypes returns all valid DNS provider types
func AllDnsProviderTypes() []DnsProviderType {
	return []DnsProviderType{
		DnsProviderTypeCloudflare,
		DnsProviderTypeDigitalOcean,
	}
}

// ParseDnsProviderType parses a string into a DnsProviderType
func ParseDnsProviderType(s string) (DnsProviderType, error) {
	p := DnsProviderType(s)
	if !p.IsValid() {
		return "", fmt.Errorf("invalid dns provider type: %s", s)
	}
	return p, nil
}

// ProviderRecord represents a DNS record from a provider (used for syncing)
type ProviderRecord struct {
	ID       string     `json:"id"`
	Type     RecordType `json:"type"`
	Name     string     `json:"name"`
	Value    string     `json:"value"`
	TTL      int        `json:"ttl"`
	Priority *int       `json:"priority,omitempty"`
	Tag      *string    `json:"tag,omitempty"`
	Weight   *int       `json:"weight,omitempty"`
	Port     *int       `json:"port,omitempty"`
	Flags    *int       `json:"flags,omitempty"`
	Comment  *string    `json:"comment,omitempty"`
	Proxied  *bool      `json:"proxied,omitempty"`
}

// DNSRecord represents a DNS record for provider operations
type DNSRecord struct {
	ID         string
	ProviderID string
	Type       RecordType
	Name       string
	Value      string
	TTL        int
	Priority   *int
	Tag        *string
	Weight     *int
	Port       *int
	Flags      *int
	Comment    *string
	Proxied    *bool
}

// IsEditable returns true if this record type can be edited by users
func (r *DNSRecord) IsEditable() bool {
	return r.Type != RecordTypeNS && r.Type != RecordTypeSOA
}

// IsDeletable returns true if this record type can be deleted by users
func (r *DNSRecord) IsDeletable() bool {
	return r.Type != RecordTypeNS && r.Type != RecordTypeSOA
}
