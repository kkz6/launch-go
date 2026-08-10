package services

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/backup/storage"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

func TestGetStorageProviderConfigAppliesS3CompatibilityDefaults(t *testing.T) {
	_, db := newBackupServiceForTest(t)
	provider := &models.StorageProvider{
		ID:       401,
		TeamID:   "team-a",
		UserID:   "user-a",
		Provider: backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{
			"endpoint": "https://objects.example.com",
			"region":   "eu-1",
			"bucket":   "nightly",
			"key":      "access",
			"secret":   "secret",
		},
	}
	require.NoError(t, db.Create(provider).Error)
	logger := zerolog.Nop()
	deps := &ServiceDeps{
		ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
			Dependencies: pkgservice.Dependencies{DB: db, Logger: &logger},
			Repos:        repositories.NewRegistry(db),
		},
	}
	service := NewStorageProviderService(deps, storage.NewFactory())

	config, err := service.GetStorageProviderConfig(context.Background(), provider.ID, provider.TeamID)
	require.NoError(t, err)
	require.Equal(t, true, config["force_path_style"])
	require.Equal(t, "https://objects.example.com", config["endpoint"])
	require.Equal(t, "access", config["access_key_id"])

	_, err = service.GetStorageProviderConfig(context.Background(), provider.ID, "other-team")
	require.Error(t, err)
}

func TestAgentBackupConfigSurvivesCorruptProviderCredentials(t *testing.T) {
	_, db := newBackupServiceForTest(t)
	provider := &models.StorageProvider{
		ID:          402,
		TeamID:      "team-a",
		UserID:      "user-a",
		Provider:    backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{"bucket": "nightly"},
	}
	require.NoError(t, db.Create(provider).Error)
	backup := &models.Backup{
		StorageProviderID: provider.ID,
		CronExpression:    "0 0 * * *",
		IncludeFiles:      `["/var/www"]`,
		ExcludeFiles:      `["/var/www/cache"]`,
		Path:              "backups",
	}
	backup.ID = "agent-backup"
	backup.ServerID = "server-a"
	backup.TeamID = "team-a"
	require.NoError(t, db.Create(backup).Error)
	require.NoError(t, db.Exec(
		"UPDATE storage_providers SET credentials = ? WHERE id = ?",
		"not-valid-encrypted-json", provider.ID,
	).Error)

	logger := zerolog.Nop()
	deps := &ServiceDeps{
		ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
			Dependencies: pkgservice.Dependencies{DB: db, Logger: &logger},
			Repos:        repositories.NewRegistry(db),
		},
	}
	registry := NewServiceRegistry(deps)

	config, err := registry.AgentConfig().GetAgentBackupConfig(
		context.Background(), backup.ID, "https://launch.example.test/hooks",
	)
	require.NoError(t, err)
	require.Equal(t, backup.ID, config.ID)
	require.Equal(t, []string{"/var/www"}, config.IncludeFiles)
	require.Equal(t, []string{"/var/www/cache"}, config.ExcludeFiles)
	require.Empty(t, config.Storage)
}

func TestStorageProviderMutationsAndReadUseTeamScope(t *testing.T) {
	_, db := newBackupServiceForTest(t)
	provider := &models.StorageProvider{
		ID:          403,
		TeamID:      "team-a",
		UserID:      "user-a",
		Provider:    backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{"bucket": "nightly"},
	}
	require.NoError(t, db.Create(provider).Error)
	logger := zerolog.Nop()
	deps := &ServiceDeps{
		ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
			Dependencies: pkgservice.Dependencies{DB: db, Logger: &logger},
			Repos:        repositories.NewRegistry(db),
		},
	}
	service := NewStorageProviderService(deps, storage.NewFactory())

	_, err := service.GetStorageProvider(context.Background(), "403", "team-a")
	require.NoError(t, err)
	_, err = service.GetStorageProvider(context.Background(), "403", "other-team")
	require.Error(t, err)
	err = service.DeleteStorageProvider(context.Background(), provider.ID, "other-team")
	require.Error(t, err)
	_, err = service.UpdateStorageProvider(
		context.Background(), provider.ID, "other-team", &dto.UpdateStorageProviderRequest{},
	)
	require.Error(t, err)
}

func TestGetStorageProviderConfigRejectsInvalidS3CredentialTypes(t *testing.T) {
	_, db := newBackupServiceForTest(t)
	provider := &models.StorageProvider{
		ID:       404,
		TeamID:   "team-a",
		UserID:   "user-a",
		Provider: backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{
			"bucket": "nightly", "key": "access", "secret": "secret",
			"force_path_style": "invalid",
		},
	}
	require.NoError(t, db.Create(provider).Error)
	logger := zerolog.Nop()
	deps := &ServiceDeps{
		ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
			Dependencies: pkgservice.Dependencies{DB: db, Logger: &logger},
			Repos:        repositories.NewRegistry(db),
		},
	}
	service := NewStorageProviderService(deps, storage.NewFactory())

	_, err := service.GetStorageProviderConfig(context.Background(), provider.ID, provider.TeamID)
	require.ErrorContains(t, err, "decode S3 credentials")
}
