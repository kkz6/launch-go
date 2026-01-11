package storage

import (
	"testing"
)

func TestFactory_Create_S3(t *testing.T) {
	factory := NewFactory()

	credentials := map[string]interface{}{
		"key":              "access-key",
		"secret":           "secret-key",
		"region":           "us-east-1",
		"bucket":           "my-bucket",
		"endpoint":         "https://s3.example.com",
		"path":             "/backups",
		"force_path_style": true,
	}

	provider, err := factory.Create("s3", credentials)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if provider.Type() != "s3" {
		t.Errorf("Type() = %s, want s3", provider.Type())
	}

	s3Provider, ok := provider.(*S3Provider)
	if !ok {
		t.Fatal("expected *S3Provider")
	}

	if s3Provider.GetRegion() != "us-east-1" {
		t.Errorf("GetRegion() = %s, want us-east-1", s3Provider.GetRegion())
	}
	if s3Provider.GetBucket() != "my-bucket" {
		t.Errorf("GetBucket() = %s, want my-bucket", s3Provider.GetBucket())
	}
}

func TestFactory_Create_Dropbox(t *testing.T) {
	factory := NewFactory()

	credentials := map[string]interface{}{
		"token": "dropbox-token",
	}

	provider, err := factory.Create("dropbox", credentials)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if provider.Type() != "dropbox" {
		t.Errorf("Type() = %s, want dropbox", provider.Type())
	}

	dropboxProvider, ok := provider.(*DropboxProvider)
	if !ok {
		t.Fatal("expected *DropboxProvider")
	}

	if dropboxProvider.GetToken() != "dropbox-token" {
		t.Errorf("GetToken() = %s, want dropbox-token", dropboxProvider.GetToken())
	}
}

func TestFactory_Create_Unknown(t *testing.T) {
	factory := NewFactory()

	_, err := factory.Create("unknown", nil)
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestFactory_CreateFromConfig(t *testing.T) {
	factory := NewFactory()

	config := map[string]interface{}{
		"type": "s3",
		"credentials": map[string]interface{}{
			"key":    "access-key",
			"secret": "secret-key",
			"region": "us-west-2",
			"bucket": "test-bucket",
		},
	}

	provider, err := factory.CreateFromConfig(config)
	if err != nil {
		t.Fatalf("CreateFromConfig() error = %v", err)
	}

	if provider.Type() != "s3" {
		t.Errorf("Type() = %s, want s3", provider.Type())
	}
}

func TestFactory_CreateFromConfig_NoCredentials(t *testing.T) {
	factory := NewFactory()

	// Config without nested credentials uses top-level config
	config := map[string]interface{}{
		"type":   "s3",
		"key":    "access-key",
		"secret": "secret-key",
		"region": "us-west-2",
		"bucket": "test-bucket",
	}

	provider, err := factory.CreateFromConfig(config)
	if err != nil {
		t.Fatalf("CreateFromConfig() error = %v", err)
	}

	if provider.Type() != "s3" {
		t.Errorf("Type() = %s, want s3", provider.Type())
	}
}

func TestFactory_CreateFromConfig_MissingType(t *testing.T) {
	factory := NewFactory()

	config := map[string]interface{}{
		"key": "access-key",
	}

	_, err := factory.CreateFromConfig(config)
	if err == nil {
		t.Error("expected error for missing type")
	}
}

func TestFactory_CreateFromConfig_InvalidType(t *testing.T) {
	factory := NewFactory()

	config := map[string]interface{}{
		"type": 123, // Invalid type (not string)
	}

	_, err := factory.CreateFromConfig(config)
	if err == nil {
		t.Error("expected error for invalid type")
	}
}
