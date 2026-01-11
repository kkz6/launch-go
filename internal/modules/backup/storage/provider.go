package storage

import (
	"context"
	"fmt"
)

// Provider defines the interface for storage providers
type Provider interface {
	// Connect tests the connection to the storage provider
	Connect(ctx context.Context) error

	// Delete removes files from the storage provider
	Delete(ctx context.Context, paths []string) error

	// GetConfigForAgent returns the configuration for the agent
	GetConfigForAgent() map[string]interface{}

	// CredentialData extracts credential data from input
	CredentialData(input map[string]interface{}) map[string]interface{}

	// Type returns the storage driver type
	Type() string
}

// Factory creates storage providers
type Factory struct{}

// NewFactory creates a new storage provider factory
func NewFactory() *Factory {
	return &Factory{}
}

// Create creates a storage provider from credentials
func (f *Factory) Create(providerType string, credentials map[string]interface{}) (Provider, error) {
	switch providerType {
	case "s3":
		return NewS3Provider(credentials), nil
	case "dropbox":
		return NewDropboxProvider(credentials), nil
	default:
		return nil, fmt.Errorf("unknown storage provider type: %s", providerType)
	}
}

// CreateFromConfig creates a storage provider from a configuration map
func (f *Factory) CreateFromConfig(config map[string]interface{}) (Provider, error) {
	providerType, ok := config["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid provider type")
	}

	credentials, ok := config["credentials"].(map[string]interface{})
	if !ok {
		credentials = config
	}

	return f.Create(providerType, credentials)
}
