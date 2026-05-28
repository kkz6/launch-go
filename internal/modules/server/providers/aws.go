package providers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
)

// AWSSSMParameterByOS maps our internal OS keys to the Canonical-published
// SSM Parameter Store names that resolve to "the current Ubuntu AMI" in any
// region. These are the recommended way to reference Ubuntu AMIs on AWS:
// Canonical updates them every time they publish a patched image, so callers
// always get the current AMI rather than a frozen ID that drifts within
// days. We keep the static fallback map in config/options.go for now but
// any new AWS image resolution should go through these parameters.
//
// API docs:
//   https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/finding-an-ami.html#finding-an-ami-parameter-store
//   https://ubuntu.com/blog/finding-ubuntu-images-on-aws
var AWSSSMParameterByOS = map[string]string{
	"ubuntu_24": "/aws/service/canonical/ubuntu/server/24.04/stable/current/amd64/hvm/ebs-gp3/ami-id",
	"ubuntu_22": "/aws/service/canonical/ubuntu/server/22.04/stable/current/amd64/hvm/ebs-gp3/ami-id",
}

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

// LookupImage verifies that an image identifier resolves to a live AMI in
// the given region. Three cases:
//
//  1. The identifier starts with "/aws/service/" — it's an SSM Parameter
//     Store name (the recommended form). Resolve it via SSM GetParameter,
//     then DescribeImages on the resolved AMI to confirm it still exists.
//  2. The identifier starts with "ami-" — it's a static AMI ID from the
//     legacy options.go map. DescribeImages directly.
//  3. Anything else — invalid.
//
// The legacy static AMI map is the exact failure mode we're trying to
// detect: Canonical deregisters older patched images regularly, so an
// AMI ID that worked last week may be gone today. The SSM parameter form
// avoids the problem entirely (always resolves to the current published
// image) but callers still using the static IDs need this verification.
//
// `region` defaults to defaultAWSValidationRegion when empty since the
// SSM parameter we use is region-replicated and any region will resolve
// it; the resolved AMI is regional though, so the same region must be
// used for DescribeImages.
func (p *AWSProvider) LookupImage(ctx context.Context, creds map[string]interface{}, identifier string) error {
	accessKey := GetStringField(creds, "access_key", "")
	secretKey := GetStringField(creds, "secret_key", "")
	region := GetStringField(creds, "region", defaultAWSValidationRegion)
	if accessKey == "" || secretKey == "" {
		return ErrInvalidCredentials
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
	)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}

	amiID := identifier
	if strings.HasPrefix(identifier, "/aws/service/") {
		// SSM parameter — resolve to an AMI ID first.
		ssmClient := ssm.NewFromConfig(cfg)
		out, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
			Name: &identifier,
		})
		if err != nil {
			return fmt.Errorf("resolve SSM parameter %q: %w", identifier, err)
		}
		if out.Parameter == nil || out.Parameter.Value == nil || *out.Parameter.Value == "" {
			return fmt.Errorf("SSM parameter %q returned empty value", identifier)
		}
		amiID = *out.Parameter.Value
	} else if !strings.HasPrefix(identifier, "ami-") {
		return fmt.Errorf("identifier %q is neither an SSM parameter (/aws/service/…) nor an AMI ID (ami-…)", identifier)
	}

	// Confirm the AMI itself is still served. DescribeImages with a single
	// owner-restricted ImageId returns the record on success, "InvalidAMIID.NotFound"
	// when the AMI has been deregistered.
	ec2Client := ec2.NewFromConfig(cfg)
	out, err := ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		ImageIds: []string{amiID},
	})
	if err != nil {
		return fmt.Errorf("DescribeImages %q in %s: %w", amiID, region, err)
	}
	if len(out.Images) == 0 {
		return fmt.Errorf("AMI %q not found in region %s", amiID, region)
	}
	return nil
}

// ResolveSSMImage returns the current AMI ID for one of our OS keys by
// asking SSM Parameter Store. Returns "" + error if the OS isn't mapped
// to an SSM parameter (yet) or the parameter fetch fails. Callers that
// want a never-fail path can chain to GetImageForRegion for the legacy
// static map as a fallback.
func (p *AWSProvider) ResolveSSMImage(ctx context.Context, creds map[string]interface{}, osKey string) (string, error) {
	param, ok := AWSSSMParameterByOS[osKey]
	if !ok {
		return "", fmt.Errorf("no SSM parameter mapped for OS %q", osKey)
	}
	accessKey := GetStringField(creds, "access_key", "")
	secretKey := GetStringField(creds, "secret_key", "")
	region := GetStringField(creds, "region", defaultAWSValidationRegion)
	if accessKey == "" || secretKey == "" {
		return "", ErrInvalidCredentials
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
	)
	if err != nil {
		return "", fmt.Errorf("load AWS config: %w", err)
	}
	client := ssm.NewFromConfig(cfg)
	out, err := client.GetParameter(ctx, &ssm.GetParameterInput{Name: &param})
	if err != nil {
		return "", fmt.Errorf("resolve SSM parameter %q: %w", param, err)
	}
	if out.Parameter == nil || out.Parameter.Value == nil {
		return "", fmt.Errorf("SSM parameter %q has no value", param)
	}
	return *out.Parameter.Value, nil
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
