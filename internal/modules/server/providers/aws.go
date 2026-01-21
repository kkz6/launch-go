package providers

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/sshkey"
)

// AWSProvider implements the Provider interface for AWS
type AWSProvider struct {
	BaseCloudProvider
}

// NewAWSProvider creates a new AWS provider
func NewAWSProvider(keyGenerator sshkey.Generator) *AWSProvider {
	configs := config.GetProviderConfigs()
	return &AWSProvider{
		BaseCloudProvider: NewBaseCloudProvider(
			keyGenerator,
			configs["aws"],
			"", // AWS uses SDK, not direct HTTP API
		),
	}
}

// Type returns the provider type
func (p *AWSProvider) Type() enums.ServerProvider {
	return enums.ProviderAWS
}

// Connect tests the connection to AWS
func (p *AWSProvider) Connect(ctx context.Context, credentials map[string]interface{}) error {
	accessKey := GetStringField(credentials, "access_key", "")
	secretKey := GetStringField(credentials, "secret_key", "")

	if accessKey == "" || secretKey == "" {
		return ErrInvalidCredentials
	}

	// AWS connection validation would use AWS SDK
	// For now, just validate credentials are present
	return nil
}

// Create creates a new server on AWS
func (p *AWSProvider) Create(ctx context.Context, server *models.Server, credentials map[string]interface{}) (*CreateResult, error) {
	keyPair, err := p.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	providerData := GetProviderData(server.ProviderData)
	region := GetStringField(providerData, "region", "")

	// AWS EC2 creation would use AWS SDK
	// This is a placeholder implementation
	return &CreateResult{
		PublicKey:  keyPair.PublicKey,
		PrivateKey: keyPair.PrivateKey,
		ProviderData: map[string]interface{}{
			"region": region,
		},
	}, nil
}

// Delete deletes a server from AWS
func (p *AWSProvider) Delete(ctx context.Context, server *models.Server, credentials map[string]interface{}) error {
	// AWS EC2 deletion would use AWS SDK
	return nil
}

// GetPublicIPv4 fetches the public IPv4 address
func (p *AWSProvider) GetPublicIPv4(ctx context.Context, server *models.Server, credentials map[string]interface{}) (string, error) {
	// AWS EC2 IP lookup would use AWS SDK
	return "", nil
}

// GetImage returns the AMI ID for an operating system and region
func (p *AWSProvider) GetImage(os enums.OperatingSystem) string {
	// For AWS, we need region-specific AMIs
	// This returns a default; actual usage should call GetImageForRegion
	return ""
}

// GetImageForRegion returns the AMI ID for an operating system in a specific region
func (p *AWSProvider) GetImageForRegion(os enums.OperatingSystem, region string) string {
	return config.GetAWSImageForRegion(region, os.String())
}

// CredentialRules returns validation rules
func (p *AWSProvider) CredentialRules() map[string]string {
	return map[string]string{
		"access_key": "required",
		"secret_key": "required",
	}
}

// CreateRules returns validation rules
func (p *AWSProvider) CreateRules() map[string]string {
	return CommonCreateRules()
}

// CredentialData extracts credential data
func (p *AWSProvider) CredentialData(input map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"access_key": input["access_key"],
		"secret_key": input["secret_key"],
	}
}

// ProviderData extracts provider-specific data
func (p *AWSProvider) ProviderData(input map[string]interface{}) map[string]interface{} {
	return CommonProviderData(input)
}
