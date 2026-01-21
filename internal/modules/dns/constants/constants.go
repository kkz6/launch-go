package constants

import "time"

// DNS providers
const (
	ProviderCloudflare   = "cloudflare"
	ProviderRoute53      = "route53"
	ProviderDigitalOcean = "digitalocean"
	ProviderLinode       = "linode"
)

// AllDNSProviders returns all supported DNS providers
var AllDNSProviders = []string{
	ProviderCloudflare,
	ProviderRoute53,
	ProviderDigitalOcean,
	ProviderLinode,
}

// Record types
const (
	RecordA     = "A"
	RecordAAAA  = "AAAA"
	RecordCNAME = "CNAME"
	RecordMX    = "MX"
	RecordTXT   = "TXT"
	RecordNS    = "NS"
	RecordSRV   = "SRV"
	RecordCAA   = "CAA"
)

// AllRecordTypes returns all supported record types
var AllRecordTypes = []string{
	RecordA,
	RecordAAAA,
	RecordCNAME,
	RecordMX,
	RecordTXT,
	RecordNS,
	RecordSRV,
	RecordCAA,
}

// Default TTL values
const (
	TTLDefault = 3600  // 1 hour
	TTLMinimum = 60    // 1 minute
	TTLMaximum = 86400 // 24 hours
)

// Propagation settings
const (
	PropagationCheckInterval = 30 * time.Second
	PropagationTimeout       = 10 * time.Minute
)

// Cloudflare specific
const (
	CloudflareProxied = true
	CloudflareMinTTL  = 1 // Auto TTL when proxied
)
