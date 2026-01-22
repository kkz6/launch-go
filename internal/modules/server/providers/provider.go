package providers

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
)

var (
	ErrConnectionFailed    = fiberutil.BadRequest("Failed to connect to provider")
	ErrProviderError       = fiberutil.BadRequest("Provider error")
	ErrInvalidCredentials  = fiberutil.BadRequest("Invalid credentials")
	ErrUnsupportedProvider = fiberutil.BadRequest("Unsupported provider")
)

// KeyPair is an alias for sshkey.KeyPair for backwards compatibility.
type KeyPair = sshkey.KeyPair

// CreateResult contains the result of server creation
type CreateResult struct {
	ProviderServerID string
	PublicIPv4       string
	PublicKey        string
	PrivateKey       string
	CPUCores         int
	MemoryMB         int
	DiskGB           int
	SSHKeyID         string
	ProviderData     map[string]interface{}
}

// Provider defines the interface for cloud server providers
type Provider interface {
	// Connect tests the connection to the provider with credentials
	Connect(ctx context.Context, credentials map[string]interface{}) error

	// Create creates a new server on the provider
	Create(ctx context.Context, server *models.Server, credentials map[string]interface{}) (*CreateResult, error)

	// Delete deletes a server from the provider
	Delete(ctx context.Context, server *models.Server, credentials map[string]interface{}) error

	// GetPublicIPv4 fetches the public IPv4 address of a server
	GetPublicIPv4(ctx context.Context, server *models.Server, credentials map[string]interface{}) (string, error)

	// Plans returns available plans for this provider
	Plans() []config.PlanOption

	// Regions returns available regions for this provider
	Regions() []config.RegionOption

	// GetImage returns the image ID for an operating system
	GetImage(os types.OperatingSystem) string

	// CredentialRules returns validation rules for credentials
	CredentialRules() map[string]string

	// CreateRules returns validation rules for server creation
	CreateRules() map[string]string

	// CredentialData extracts credential data from input
	CredentialData(input map[string]interface{}) map[string]interface{}

	// ProviderData extracts provider-specific data from input
	ProviderData(input map[string]interface{}) map[string]interface{}

	// Type returns the provider type
	Type() types.ServerProvider
}

// Factory creates server provider instances
type Factory struct {
	keyGenerator sshkey.Generator
}

// NewFactory creates a new provider factory
func NewFactory(keyGenerator sshkey.Generator) *Factory {
	return &Factory{
		keyGenerator: keyGenerator,
	}
}

// Create creates a provider instance based on provider type
func (f *Factory) Create(providerType types.ServerProvider) (Provider, error) {
	switch providerType {
	case types.ProviderDigitalOcean:
		return NewDigitalOceanProvider(f.keyGenerator), nil
	case types.ProviderHetzner:
		return NewHetznerProvider(f.keyGenerator), nil
	case types.ProviderLinode:
		return NewLinodeProvider(f.keyGenerator), nil
	case types.ProviderVultr:
		return NewVultrProvider(f.keyGenerator), nil
	case types.ProviderAWS:
		return NewAWSProvider(f.keyGenerator), nil
	case types.ProviderCustom:
		return NewCustomProvider(f.keyGenerator), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerType)
	}
}

// CreateFromServer creates a provider instance for a server
func (f *Factory) CreateFromServer(server *models.Server) (Provider, error) {
	return f.Create(server.Provider)
}
