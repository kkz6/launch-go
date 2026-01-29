package storage

import (
	"context"
	"fmt"
)

// S3Provider implements the Provider interface for S3-compatible storage
type S3Provider struct {
	endpoint        string
	accessKeyID     string
	secretAccessKey string
	region          string
	bucket          string
	path            string
	forcePathStyle  bool
}

// NewS3Provider creates a new S3 storage provider
func NewS3Provider(credentials map[string]interface{}) *S3Provider {
	p := &S3Provider{}

	if endpoint, ok := credentials["endpoint"].(string); ok {
		p.endpoint = endpoint
	}
	if key, ok := credentials["key"].(string); ok {
		p.accessKeyID = key
	}
	if secret, ok := credentials["secret"].(string); ok {
		p.secretAccessKey = secret
	}
	if region, ok := credentials["region"].(string); ok {
		p.region = region
	}
	if bucket, ok := credentials["bucket"].(string); ok {
		p.bucket = bucket
	}
	if path, ok := credentials["path"].(string); ok {
		p.path = path
	}
	if fps, ok := credentials["force_path_style"].(bool); ok {
		p.forcePathStyle = fps
	}

	return p
}

// Connect tests the connection to the S3 storage
func (p *S3Provider) Connect(ctx context.Context) error {
	// In production, this would use the AWS SDK to list buckets
	// For now, we validate that required credentials are present
	if p.accessKeyID == "" {
		return fmt.Errorf("S3 access key ID is required")
	}
	if p.secretAccessKey == "" {
		return fmt.Errorf("S3 secret access key is required")
	}
	if p.region == "" {
		return fmt.Errorf("S3 region is required")
	}
	if p.bucket == "" {
		return fmt.Errorf("S3 bucket is required")
	}

	// In production implementation:
	// cfg, err := config.LoadDefaultConfig(ctx,
	//     config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(p.accessKeyID, p.secretAccessKey, "")),
	//     config.WithRegion(p.region),
	// )
	// client := s3.NewFromConfig(cfg)
	// _, err = client.ListBuckets(ctx, &s3.ListBucketsInput{})
	// return err

	return nil
}

// Delete removes files from S3 storage
func (p *S3Provider) Delete(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	// In production implementation:
	// for _, path := range paths {
	//     _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
	//         Bucket: aws.String(p.bucket),
	//         Key:    aws.String(path),
	//     })
	//     if err != nil {
	//         return err
	//     }
	// }

	return nil
}

// GetConfigForAgent returns the configuration for the agent
func (p *S3Provider) GetConfigForAgent() map[string]interface{} {
	return map[string]interface{}{
		"endpoint":          p.getAPIURL(),
		"region":            p.region,
		"bucket":            p.bucket,
		"path":              p.path,
		"force_path_style":  p.forcePathStyle,
		"access_key_id":     p.accessKeyID,
		"secret_access_key": p.secretAccessKey,
	}
}

// CredentialData extracts credential data from input
func (p *S3Provider) CredentialData(input map[string]interface{}) map[string]interface{} {
	data := make(map[string]interface{})

	if endpoint, ok := input["endpoint"].(string); ok {
		data["endpoint"] = endpoint
	} else {
		data["endpoint"] = ""
	}
	if key, ok := input["key"].(string); ok {
		data["key"] = key
	}
	if secret, ok := input["secret"].(string); ok {
		data["secret"] = secret
	}
	if region, ok := input["region"].(string); ok {
		data["region"] = region
	}
	if bucket, ok := input["bucket"].(string); ok {
		data["bucket"] = bucket
	}
	if path, ok := input["path"].(string); ok {
		data["path"] = path
	} else {
		data["path"] = ""
	}
	if fps, ok := input["force_path_style"].(bool); ok {
		data["force_path_style"] = fps
	} else {
		data["force_path_style"] = false
	}

	return data
}

// Type returns the storage driver type
func (p *S3Provider) Type() string {
	return "s3"
}

// getAPIURL returns the API URL for the S3 endpoint
func (p *S3Provider) getAPIURL() string {
	if p.endpoint != "" {
		return p.endpoint
	}
	return fmt.Sprintf("https://s3.%s.amazonaws.com", p.region)
}

// GetEndpoint returns the configured endpoint
func (p *S3Provider) GetEndpoint() string {
	return p.endpoint
}

// GetRegion returns the configured region
func (p *S3Provider) GetRegion() string {
	return p.region
}

// GetBucket returns the configured bucket
func (p *S3Provider) GetBucket() string {
	return p.bucket
}

// GetPath returns the configured path
func (p *S3Provider) GetPath() string {
	return p.path
}

// IsForcePathStyle returns whether force path style is enabled
func (p *S3Provider) IsForcePathStyle() bool {
	return p.forcePathStyle
}
