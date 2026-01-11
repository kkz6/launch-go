package storage

import (
	"context"
	"testing"
)

func TestNewS3Provider(t *testing.T) {
	credentials := map[string]interface{}{
		"endpoint":         "https://s3.example.com",
		"key":              "access-key",
		"secret":           "secret-key",
		"region":           "us-east-1",
		"bucket":           "my-bucket",
		"path":             "/backups",
		"force_path_style": true,
	}

	provider := NewS3Provider(credentials)

	if provider.GetEndpoint() != "https://s3.example.com" {
		t.Errorf("GetEndpoint() = %s, want https://s3.example.com", provider.GetEndpoint())
	}
	if provider.accessKeyID != "access-key" {
		t.Errorf("accessKeyID = %s, want access-key", provider.accessKeyID)
	}
	if provider.secretAccessKey != "secret-key" {
		t.Errorf("secretAccessKey = %s, want secret-key", provider.secretAccessKey)
	}
	if provider.GetRegion() != "us-east-1" {
		t.Errorf("GetRegion() = %s, want us-east-1", provider.GetRegion())
	}
	if provider.GetBucket() != "my-bucket" {
		t.Errorf("GetBucket() = %s, want my-bucket", provider.GetBucket())
	}
	if provider.GetPath() != "/backups" {
		t.Errorf("GetPath() = %s, want /backups", provider.GetPath())
	}
	if !provider.IsForcePathStyle() {
		t.Error("expected ForcePathStyle to be true")
	}
}

func TestNewS3Provider_Empty(t *testing.T) {
	provider := NewS3Provider(nil)

	if provider.GetEndpoint() != "" {
		t.Errorf("GetEndpoint() = %s, want empty", provider.GetEndpoint())
	}
	if provider.accessKeyID != "" {
		t.Errorf("accessKeyID = %s, want empty", provider.accessKeyID)
	}
}

func TestS3Provider_Connect_MissingCredentials(t *testing.T) {
	tests := []struct {
		name        string
		credentials map[string]interface{}
		wantErr     string
	}{
		{
			name:        "missing key",
			credentials: map[string]interface{}{"secret": "s", "region": "r", "bucket": "b"},
			wantErr:     "S3 access key ID is required",
		},
		{
			name:        "missing secret",
			credentials: map[string]interface{}{"key": "k", "region": "r", "bucket": "b"},
			wantErr:     "S3 secret access key is required",
		},
		{
			name:        "missing region",
			credentials: map[string]interface{}{"key": "k", "secret": "s", "bucket": "b"},
			wantErr:     "S3 region is required",
		},
		{
			name:        "missing bucket",
			credentials: map[string]interface{}{"key": "k", "secret": "s", "region": "r"},
			wantErr:     "S3 bucket is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewS3Provider(tt.credentials)
			err := provider.Connect(context.Background())
			if err == nil {
				t.Error("expected error")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("error = %s, want %s", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestS3Provider_Connect_Success(t *testing.T) {
	credentials := map[string]interface{}{
		"key":    "access-key",
		"secret": "secret-key",
		"region": "us-east-1",
		"bucket": "my-bucket",
	}

	provider := NewS3Provider(credentials)
	err := provider.Connect(context.Background())
	if err != nil {
		t.Errorf("Connect() error = %v", err)
	}
}

func TestS3Provider_Delete(t *testing.T) {
	credentials := map[string]interface{}{
		"key":    "access-key",
		"secret": "secret-key",
		"region": "us-east-1",
		"bucket": "my-bucket",
	}

	provider := NewS3Provider(credentials)

	// Empty paths should not error
	err := provider.Delete(context.Background(), []string{})
	if err != nil {
		t.Errorf("Delete() with empty paths error = %v", err)
	}

	// Non-empty paths (mock implementation just returns nil)
	err = provider.Delete(context.Background(), []string{"/path/to/file1", "/path/to/file2"})
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}
}

func TestS3Provider_GetConfigForAgent(t *testing.T) {
	credentials := map[string]interface{}{
		"endpoint":         "https://s3.example.com",
		"key":              "access-key",
		"secret":           "secret-key",
		"region":           "us-east-1",
		"bucket":           "my-bucket",
		"path":             "/backups",
		"force_path_style": true,
	}

	provider := NewS3Provider(credentials)
	config := provider.GetConfigForAgent()

	if config["endpoint"] != "https://s3.example.com" {
		t.Errorf("endpoint = %v, want https://s3.example.com", config["endpoint"])
	}
	if config["region"] != "us-east-1" {
		t.Errorf("region = %v, want us-east-1", config["region"])
	}
	if config["bucket"] != "my-bucket" {
		t.Errorf("bucket = %v, want my-bucket", config["bucket"])
	}
	if config["path"] != "/backups" {
		t.Errorf("path = %v, want /backups", config["path"])
	}
	if config["force_path_style"] != true {
		t.Errorf("force_path_style = %v, want true", config["force_path_style"])
	}
	if config["access_key_id"] != "access-key" {
		t.Errorf("access_key_id = %v, want access-key", config["access_key_id"])
	}
	if config["secret_access_key"] != "secret-key" {
		t.Errorf("secret_access_key = %v, want secret-key", config["secret_access_key"])
	}
}

func TestS3Provider_GetConfigForAgent_DefaultEndpoint(t *testing.T) {
	credentials := map[string]interface{}{
		"key":    "access-key",
		"secret": "secret-key",
		"region": "us-west-2",
		"bucket": "my-bucket",
	}

	provider := NewS3Provider(credentials)
	config := provider.GetConfigForAgent()

	expected := "https://s3.us-west-2.amazonaws.com"
	if config["endpoint"] != expected {
		t.Errorf("endpoint = %v, want %s", config["endpoint"], expected)
	}
}

func TestS3Provider_CredentialData(t *testing.T) {
	provider := NewS3Provider(nil)

	input := map[string]interface{}{
		"endpoint":         "https://custom.s3.com",
		"key":              "my-key",
		"secret":           "my-secret",
		"region":           "eu-west-1",
		"bucket":           "test-bucket",
		"path":             "/data",
		"force_path_style": true,
	}

	data := provider.CredentialData(input)

	if data["endpoint"] != "https://custom.s3.com" {
		t.Errorf("endpoint = %v, want https://custom.s3.com", data["endpoint"])
	}
	if data["key"] != "my-key" {
		t.Errorf("key = %v, want my-key", data["key"])
	}
	if data["secret"] != "my-secret" {
		t.Errorf("secret = %v, want my-secret", data["secret"])
	}
	if data["region"] != "eu-west-1" {
		t.Errorf("region = %v, want eu-west-1", data["region"])
	}
	if data["bucket"] != "test-bucket" {
		t.Errorf("bucket = %v, want test-bucket", data["bucket"])
	}
	if data["path"] != "/data" {
		t.Errorf("path = %v, want /data", data["path"])
	}
	if data["force_path_style"] != true {
		t.Errorf("force_path_style = %v, want true", data["force_path_style"])
	}
}

func TestS3Provider_CredentialData_Defaults(t *testing.T) {
	provider := NewS3Provider(nil)

	input := map[string]interface{}{
		"key":    "my-key",
		"secret": "my-secret",
		"region": "us-east-1",
		"bucket": "bucket",
	}

	data := provider.CredentialData(input)

	if data["endpoint"] != "" {
		t.Errorf("endpoint = %v, want empty", data["endpoint"])
	}
	if data["path"] != "" {
		t.Errorf("path = %v, want empty", data["path"])
	}
	if data["force_path_style"] != false {
		t.Errorf("force_path_style = %v, want false", data["force_path_style"])
	}
}

func TestS3Provider_Type(t *testing.T) {
	provider := NewS3Provider(nil)
	if provider.Type() != "s3" {
		t.Errorf("Type() = %s, want s3", provider.Type())
	}
}

func TestS3Provider_getAPIURL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		region   string
		want     string
	}{
		{
			name:     "custom endpoint",
			endpoint: "https://custom.s3.com",
			region:   "us-east-1",
			want:     "https://custom.s3.com",
		},
		{
			name:     "default AWS endpoint",
			endpoint: "",
			region:   "us-west-2",
			want:     "https://s3.us-west-2.amazonaws.com",
		},
		{
			name:     "eu region",
			endpoint: "",
			region:   "eu-central-1",
			want:     "https://s3.eu-central-1.amazonaws.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentials := map[string]interface{}{
				"endpoint": tt.endpoint,
				"region":   tt.region,
			}
			provider := NewS3Provider(credentials)
			if got := provider.getAPIURL(); got != tt.want {
				t.Errorf("getAPIURL() = %s, want %s", got, tt.want)
			}
		})
	}
}
