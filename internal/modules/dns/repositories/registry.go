package repositories

import (
	"gorm.io/gorm"
)

// Registry holds all DNS module repositories
type Registry struct {
	provider  *DomainProviderRepository
	domain    *DomainRepository
	dnsRecord *DnsRecordRepository
}

// NewRegistry creates all repositories
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		provider:  NewDomainProviderRepository(db),
		domain:    NewDomainRepository(db),
		dnsRecord: NewDnsRecordRepository(db),
	}
}

// Provider returns the domain provider repository
func (r *Registry) Provider() *DomainProviderRepository { return r.provider }

// Domain returns the domain repository
func (r *Registry) Domain() *DomainRepository { return r.domain }

// DnsRecord returns the DNS record repository
func (r *Registry) DnsRecord() *DnsRecordRepository { return r.dnsRecord }
