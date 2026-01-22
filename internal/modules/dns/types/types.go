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
// DnsProvider
// =============================================================================

// DnsProvider represents a DNS provider
type DnsProvider string

const (
	DnsProviderCloudflare   DnsProvider = "cloudflare"
	DnsProviderDigitalOcean DnsProvider = "digitalocean"
)

var allDnsProviders = []DnsProvider{
	DnsProviderCloudflare,
	DnsProviderDigitalOcean,
}

// AllDnsProviders returns all valid DNS providers
func AllDnsProviders() []DnsProvider {
	return allDnsProviders
}

// String returns the string representation of DnsProvider
func (p DnsProvider) String() string {
	return string(p)
}

// Label returns a human-readable label for the provider
func (p DnsProvider) Label() string {
	switch p {
	case DnsProviderCloudflare:
		return "Cloudflare"
	case DnsProviderDigitalOcean:
		return "DigitalOcean"
	default:
		return string(p)
	}
}

// IsValid checks if the DnsProvider is valid
func (p DnsProvider) IsValid() bool {
	switch p {
	case DnsProviderCloudflare, DnsProviderDigitalOcean:
		return true
	}

	return false
}

// Value implements driver.Valuer for database storage
func (p DnsProvider) Value() (driver.Value, error) {
	return enumtypes.Value(p)
}

// Scan implements sql.Scanner for database retrieval
func (p *DnsProvider) Scan(value any) error {
	return enumtypes.Scan(p, value)
}

// ToProviderType converts DnsProvider to providers.DnsProviderType
func (p DnsProvider) ToProviderType() providers.DnsProviderType {
	return providers.DnsProviderType(p)
}

// ParseDnsProvider parses a string into a DnsProvider
func ParseDnsProvider(s string) (DnsProvider, error) {
	p := DnsProvider(s)
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
	switch s {
	case SyncStatusPending:
		return "Pending"
	case SyncStatusSyncing:
		return "Syncing"
	case SyncStatusCompleted:
		return "Completed"
	case SyncStatusFailed:
		return "Failed"
	default:
		return string(s)
	}
}

// IsValid checks if the SyncStatus is valid
func (s SyncStatus) IsValid() bool {
	switch s {
	case SyncStatusPending, SyncStatusSyncing, SyncStatusCompleted, SyncStatusFailed:
		return true
	}

	return false
}

// Value implements driver.Valuer for database storage
func (s SyncStatus) Value() (driver.Value, error) {
	return enumtypes.Value(s)
}

// Scan implements sql.Scanner for database retrieval
func (s *SyncStatus) Scan(value any) error {
	return enumtypes.Scan(s, value)
}
