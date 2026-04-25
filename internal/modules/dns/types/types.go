// Package types contains all type definitions for the dns module
package types

import (
	"database/sql/driver"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/dns/providers"
	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// =============================================================================
// RecordType
// =============================================================================

// RecordType is an alias to providers.RecordType for use in GORM models
type RecordType = providers.RecordType

// Record type constants (re-exported from providers package)
const (
	RecordTypeA     = providers.RecordTypeA
	RecordTypeAAAA  = providers.RecordTypeAAAA
	RecordTypeCNAME = providers.RecordTypeCNAME
	RecordTypeMX    = providers.RecordTypeMX
	RecordTypeNS    = providers.RecordTypeNS
	RecordTypeSRV   = providers.RecordTypeSRV
	RecordTypeTXT   = providers.RecordTypeTXT
	RecordTypeSOA   = providers.RecordTypeSOA
	RecordTypeCAA   = providers.RecordTypeCAA
)

// AllRecordTypes returns all valid record types
func AllRecordTypes() []RecordType {
	return providers.AllRecordTypes()
}

// ParseRecordType parses a string into a RecordType
func ParseRecordType(s string) (RecordType, error) {
	return providers.ParseRecordType(s)
}

// =============================================================================
// DNSProvider
// =============================================================================

// DNSProvider represents a DNS provider
type DNSProvider string

const (
	DNSProviderCloudflare   DNSProvider = "cloudflare"
	DNSProviderDigitalOcean DNSProvider = "digitalocean"
)

var allDNSProviders = []DNSProvider{
	DNSProviderCloudflare,
	DNSProviderDigitalOcean,
}

var dnsProviderLabels = map[DNSProvider]string{
	DNSProviderCloudflare:   "Cloudflare",
	DNSProviderDigitalOcean: "DigitalOcean",
}

// AllDNSProviders returns all valid DNS providers
func AllDNSProviders() []DNSProvider {
	return allDNSProviders
}

// String returns the string representation of DNSProvider
func (p DNSProvider) String() string {
	return string(p)
}

// Label returns a human-readable label for the provider
func (p DNSProvider) Label() string {
	return enumtypes.Label(p, dnsProviderLabels, string(p))
}

// IsValid checks if the DNSProvider is valid
func (p DNSProvider) IsValid() bool {
	return enumtypes.IsValid(p, allDNSProviders...)
}

// Value implements driver.Valuer for database storage
func (p DNSProvider) Value() (driver.Value, error) {
	return enumtypes.Value(p)
}

// Scan implements sql.Scanner for database retrieval
func (p *DNSProvider) Scan(value any) error {
	return enumtypes.Scan(p, value)
}

// ToProviderType converts DNSProvider to providers.DNSProviderType
func (p DNSProvider) ToProviderType() providers.DNSProviderType {
	return providers.DNSProviderType(p)
}

// ParseDNSProvider parses a string into a DNSProvider
func ParseDNSProvider(s string) (DNSProvider, error) {
	p := DNSProvider(s)
	if !p.IsValid() {
		return "", fmt.Errorf("invalid dns provider: %s", s)
	}

	return p, nil
}

// =============================================================================
// SyncStatus
// =============================================================================

// SyncStatus represents the synchronization status of a domain provider
type SyncStatus string

const (
	SyncStatusPending   SyncStatus = "pending"
	SyncStatusSyncing   SyncStatus = "syncing"
	SyncStatusCompleted SyncStatus = "completed"
	SyncStatusFailed    SyncStatus = "failed"
)

var allSyncStatuses = []SyncStatus{
	SyncStatusPending,
	SyncStatusSyncing,
	SyncStatusCompleted,
	SyncStatusFailed,
}

var syncStatusLabels = map[SyncStatus]string{
	SyncStatusPending:   "Pending",
	SyncStatusSyncing:   "Syncing",
	SyncStatusCompleted: "Completed",
	SyncStatusFailed:    "Failed",
}

// AllSyncStatuses returns all valid sync statuses
func AllSyncStatuses() []SyncStatus {
	return allSyncStatuses
}

// String returns the string representation of SyncStatus
func (s SyncStatus) String() string {
	return string(s)
}

// Label returns a human-readable label for the sync status
func (s SyncStatus) Label() string {
	return enumtypes.Label(s, syncStatusLabels, string(s))
}

// IsValid checks if the SyncStatus is valid
func (s SyncStatus) IsValid() bool {
	return enumtypes.IsValid(s, allSyncStatuses...)
}

// Value implements driver.Valuer for database storage
func (s SyncStatus) Value() (driver.Value, error) {
	return enumtypes.Value(s)
}

// Scan implements sql.Scanner for database retrieval
func (s *SyncStatus) Scan(value any) error {
	return enumtypes.Scan(s, value)
}
