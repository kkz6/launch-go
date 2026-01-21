package providers

import (
	"context"
	"fmt"
	"strings"
)

// Provider defines the interface for DNS providers
type Provider interface {
	// Name returns the provider name
	Name() string

	// SetDomain sets the domain to operate on
	SetDomain(domain string) Provider

	// GetDomain returns the current domain
	GetDomain() string

	// SetCredentials sets the provider credentials
	SetCredentials(credentials map[string]string) Provider

	// ValidateCredentials validates the provider credentials
	ValidateCredentials(ctx context.Context) error

	// AddDomain adds a new domain to the provider
	AddDomain(ctx context.Context, domainName string) (string, error)

	// DeleteDomain deletes a domain from the provider
	DeleteDomain(ctx context.Context, domainName string) error

	// ListDomains lists all domains in the provider
	ListDomains(ctx context.Context) (map[string]string, error)

	// GetNameservers returns the nameservers for the current domain
	GetNameservers(ctx context.Context) ([]string, error)

	// ListRecords lists all DNS records for the current domain
	ListRecords(ctx context.Context) ([]ProviderRecord, error)

	// AddRecord adds a DNS record to the current domain
	AddRecord(ctx context.Context, record *DNSRecord) (string, error)

	// UpdateRecord updates a DNS record in the current domain
	UpdateRecord(ctx context.Context, record *DNSRecord) error

	// DeleteRecord deletes a DNS record from the current domain
	DeleteRecord(ctx context.Context, record *DNSRecord) error
}

// BaseProvider provides common functionality for DNS providers
type BaseProvider struct {
	domain      string
	credentials map[string]string
}

// SetDomain sets the domain to operate on
func (p *BaseProvider) SetDomain(domain string) {
	p.domain = domain
}

// GetDomain returns the current domain
func (p *BaseProvider) GetDomain() string {
	return p.domain
}

// SetCredentials sets the provider credentials
func (p *BaseProvider) SetCredentials(credentials map[string]string) {
	p.credentials = credentials
}

// GetCredentials returns the provider credentials
func (p *BaseProvider) GetCredentials() map[string]string {
	return p.credentials
}

// GetToken returns the API token from credentials
func (p *BaseProvider) GetToken() string {
	if p.credentials == nil {
		return ""
	}
	return p.credentials["token"]
}

// WithTrailingDot adds a trailing dot to a value if it's not "@"
func WithTrailingDot(value string) string {
	if value == "@" {
		return value
	}
	if !strings.HasSuffix(value, ".") {
		return value + "."
	}
	return value
}

// NewProvider creates a new provider based on the provider type
func NewProvider(providerType DnsProviderType, credentials map[string]string, additionalData map[string]interface{}) (Provider, error) {
	switch providerType {
	case DnsProviderTypeCloudflare:
		accountID := ""
		if additionalData != nil {
			if id, ok := additionalData["account_id"].(string); ok {
				accountID = id
			}
		}
		return NewCloudflareProvider(credentials, accountID), nil
	case DnsProviderTypeDigitalOcean:
		return NewDigitalOceanProvider(credentials), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerType)
	}
}

// ProviderError represents an error from a DNS provider
type ProviderError struct {
	Provider string
	Code     int
	Message  string
	Err      error
}

// Error implements the error interface
func (e *ProviderError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s provider error (code %d): %s: %v", e.Provider, e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s provider error (code %d): %s", e.Provider, e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a new ProviderError
func NewProviderError(provider string, code int, message string, err error) *ProviderError {
	return &ProviderError{
		Provider: provider,
		Code:     code,
		Message:  message,
		Err:      err,
	}
}
