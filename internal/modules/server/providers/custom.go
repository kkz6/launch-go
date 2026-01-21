package providers

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/sshkey"
)

// CustomProvider implements the Provider interface for custom/self-managed servers
type CustomProvider struct {
	BaseCloudProvider
}

// NewCustomProvider creates a new Custom provider
func NewCustomProvider(keyGenerator sshkey.Generator) *CustomProvider {
	return &CustomProvider{
		BaseCloudProvider: NewBaseCloudProvider(
			keyGenerator,
			config.ProviderConfig{}, // No plans/regions for custom
			"",                      // No API URL for custom
		),
	}
}

// Type returns the provider type
func (p *CustomProvider) Type() enums.ServerProvider {
	return enums.ProviderCustom
}

// Connect always returns true for custom providers
func (p *CustomProvider) Connect(ctx context.Context, credentials map[string]interface{}) error {
	return nil
}

// Create generates SSH keys for a custom server
func (p *CustomProvider) Create(ctx context.Context, server *models.Server, credentials map[string]interface{}) (*CreateResult, error) {
	keyPair, err := p.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	return &CreateResult{
		PublicKey:    keyPair.PublicKey,
		PrivateKey:   keyPair.PrivateKey,
		ProviderData: map[string]interface{}{},
	}, nil
}

// Delete does nothing for custom providers
func (p *CustomProvider) Delete(ctx context.Context, server *models.Server, credentials map[string]interface{}) error {
	return nil
}

// GetPublicIPv4 returns nil for custom providers (IP is user-provided)
func (p *CustomProvider) GetPublicIPv4(ctx context.Context, server *models.Server, credentials map[string]interface{}) (string, error) {
	return "", nil
}

// GetImage returns empty for custom providers
func (p *CustomProvider) GetImage(os enums.OperatingSystem) string {
	return ""
}

// Plans returns empty for custom providers
func (p *CustomProvider) Plans() []config.PlanOption {
	return []config.PlanOption{}
}

// Regions returns empty for custom providers
func (p *CustomProvider) Regions() []config.RegionOption {
	return []config.RegionOption{}
}

// CredentialRules returns empty rules for custom providers
func (p *CustomProvider) CredentialRules() map[string]string {
	return map[string]string{}
}

// CreateRules returns validation rules for custom server creation
func (p *CustomProvider) CreateRules() map[string]string {
	return map[string]string{
		"ip":   "required",
		"port": "required",
	}
}

// CredentialData returns empty for custom providers
func (p *CustomProvider) CredentialData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{}
}

// ProviderData returns empty for custom providers
func (p *CustomProvider) ProviderData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{}
}
