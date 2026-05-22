package providers

import (
	"context"
	"errors"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
)

// defaultAWSValidationRegion is used for STS GetCallerIdentity when the
// caller didn't supply a region in their credentials. STS is region-agnostic
// in practice — any region works — but the AWS SDK insists on one being set.
const defaultAWSValidationRegion = "us-east-1"

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
func (p *AWSProvider) Type() types.ServerProvider {
	return types.ProviderAWS
}

// Connect tests the connection to AWS by issuing an STS GetCallerIdentity
// call with the supplied credentials. STS rejects invalid or expired keys
// with a 4xx response — anything other than a clean success is mapped to
// ErrInvalidCredentials so the UI surfaces a 400 with the same shape as the
// other providers (DO/Hetzner/Linode/Vultr).
func (p *AWSProvider) Connect(ctx context.Context, creds map[string]interface{}) error {
	accessKey := GetStringField(creds, "access_key", "")
	secretKey := GetStringField(creds, "secret_key", "")
	region := GetStringField(creds, "region", defaultAWSValidationRegion)

	if accessKey == "" || secretKey == "" {
		return ErrInvalidCredentials
	}

	// Translate any failure from the STS shim into ErrInvalidCredentials so
	// the policy "couldn't prove valid → reject" lives in one place and the
	// shim itself stays focused on the round-trip. Network/DNS errors are
	// treated the same as auth rejections — we don't silently save creds we
	// can't verify.
	if err := validateAWSCredentials(ctx, accessKey, secretKey, region); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// validateAWSCredentials performs the actual STS round-trip. Kept as a
// package var so tests can swap it out for a stub without hitting AWS.
var validateAWSCredentials = func(ctx context.Context, accessKey, secretKey, region string) error {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
	)
	if err != nil {
		return err
	}

	client := sts.NewFromConfig(cfg)
	_, err = client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		// Distinguishing transport vs response errors isn't needed today —
		// Connect maps everything to ErrInvalidCredentials. Pattern is kept
		// so a future caller (e.g. a richer "test connection" endpoint) can
		// surface the underlying smithy response details.
		var respErr *smithyhttp.ResponseError
		if errors.As(err, &respErr) {
			return err
		}
		return err
	}
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
func (p *AWSProvider) GetImage(os types.OperatingSystem) string {
	// For AWS, we need region-specific AMIs
	// This returns a default; actual usage should call GetImageForRegion
	return ""
}

// GetImageForRegion returns the AMI ID for an operating system in a specific region
func (p *AWSProvider) GetImageForRegion(os types.OperatingSystem, region string) string {
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
